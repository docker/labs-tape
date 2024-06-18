package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	kimage "sigs.k8s.io/kustomize/api/image"

	"github.com/errordeveloper/tape/attest"
	"github.com/errordeveloper/tape/attest/manifest"
	"github.com/errordeveloper/tape/manifest/imagecopier"
	"github.com/errordeveloper/tape/manifest/imageresolver"
	"github.com/errordeveloper/tape/manifest/imagescanner"
	"github.com/errordeveloper/tape/manifest/loader"
	"github.com/errordeveloper/tape/manifest/packager"
	"github.com/errordeveloper/tape/manifest/updater"
	"github.com/errordeveloper/tape/oci"
)

type TapePackageCommand struct {
	tape *TapeCommand
	InputManifestDirOptions

	// WithImages  map[string]string `short:"I" long:"with-images" required:"false" description:"Names of new images to use instead of what specified in the manifests"`
	OutputImage string `short:"O" long:"output-image" required:"true" description:"Name of the image to push"`

	// TODO: implement
	// Push bool `short:"P" long:"push" description:"Push the resulting image to the registry"`
}

func (c *TapePackageCommand) ValidateFlags() error {
	name, tag, digest := kimage.Split(c.OutputImage)

	invalidOutputImageErr := func(reason string, values ...interface{}) error {
		return fmt.Errorf("invalid output image name %q: "+reason, values...)
	}

	if tag != "" {
		return invalidOutputImageErr("tag shouldn't be specified", c.OutputImage)
	}
	if digest != "" {
		return invalidOutputImageErr("digest shouldn't be specified", c.OutputImage)
	}
	if name == "" {
		return invalidOutputImageErr("name must not be empty", name)
	}
	if strings.ToLower(name) != name {
		return invalidOutputImageErr("must not contain upper case characters", name)
	}

	return nil
}

func (c *TapePackageCommand) Execute(args []string) error {
	ctx := context.WithValue(c.tape.ctx, "command", "package")
	if len(args) != 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}

	if err := c.tape.Init(); err != nil {
		return err
	}

	if err := c.ValidateFlags(); err != nil {
		return err
	}

	loader := loader.NewRecursiveManifestDirectoryLoader(c.ManifestDir)
	if err := loader.Load(); err != nil {
		return fmt.Errorf("failed to load manifests: %w", err)
	}
	c.tape.log.Debugf("loaded manifests: %v", loader.Paths())

	repoDetected, attreg, err := attest.DetectVCS(c.ManifestDir)
	if err != nil {
		return err
	}
	/// baseDir := c.ManifestDir
	if vcsSummary := attreg.BaseDirSummary(); repoDetected && vcsSummary != nil {
		// baseDir = vcsSummary.Common().Path
		summaryJSON, err := json.Marshal(vcsSummary.Full())
		if err != nil {
			return err
		}
		c.tape.log.Infof("VCS info for %q: %s", c.ManifestDir, summaryJSON)
	} else {
		c.tape.log.Warnf("path %q is not in VCS", c.ManifestDir)
	}

	scanner := imagescanner.NewDefaultImageScanner()
	scanner.WithProvinanceAttestor(attreg)

	if err := scanner.Scan(loader.RelPaths()); err != nil {
		return fmt.Errorf("failed to scan images: %w", err)
	}

	images := scanner.GetImages()
	c.tape.log.Debugf("found images: %#v", images.Items())

	if err := attreg.AssociateCoreStatements(); err != nil {
		return err
	}

	if err := attreg.AssociateStatements(manifest.MakeOriginalImageRefStatements(images)...); err != nil {
		return err
	}

	client := oci.NewClient(nil)
	// TODO: use client.LoginWithCredentials() and/or other options
	// TODO: integrate with docker-credential-helpers

	resolver := imageresolver.NewRegistryResolver(client)

	copier := imagecopier.NewRegistryCopier(client, c.OutputImage)

	c.tape.log.Info("resolving image digests")
	if err := resolver.ResolveDigests(ctx, images); err != nil {
		return fmt.Errorf("failed to resolve digests: %w", err)
	}

	if err := images.Dedup(); err != nil {
		return fmt.Errorf("failed to dedup images: %w", err)
	}

	if err := attreg.AssociateStatements(manifest.MakeResovedImageRefStatements(images)...); err != nil {
		return err
	}

	c.tape.log.Info("resolving related images")
	related, err := resolver.FindRelatedTags(ctx, images)
	if err != nil {
		return fmt.Errorf("failed to find related tags: %w", err)
	}

	_, relatedToManifests, err := resolver.FindRelatedFromIndecies(ctx, images, nil)
	if err != nil {
		return fmt.Errorf("failed to find images related to manifests: %w", err)
	}

	c.tape.log.Info("copying images")

	imageRefs, err := copier.CopyImages(ctx, images, related, relatedToManifests)
	if err != nil {
		return fmt.Errorf("failed to copy images: %w", err)
	}
	c.tape.log.Infof("copied images: %s", strings.Join(imageRefs, ", "))

	c.tape.log.Info("updating manifest files")

	updater := updater.NewFileUpdater()
	if err := updater.Update(images); err != nil {
		return fmt.Errorf("failed to update manifest files: %w", err)
	}
	attreg.RegisterMutated(updater.Mutations())
	scanner.Reset()
	if err := scanner.Scan(loader.RelPaths()); err != nil {
		return fmt.Errorf("failed to scan updated manifest files: %w", err)
	}
	replacedImages := scanner.GetImages()
	replacedImages.Dedup()

	if err := attreg.AssociateStatements(manifest.MakeReplacedImageRefStatements(replacedImages)...); err != nil {
		return err
	}

	c.tape.log.DebugFn(func() []interface{} {
		buf := bytes.NewBuffer(make([]byte, 0, 1024))
		base64 := base64.NewEncoder(base64.StdEncoding, buf)
		if err := attreg.EncodeAllAttestations(base64); err != nil {
			return []interface{}{"failed to encode attestations", err}
		}
		if err := base64.Close(); err != nil {
			return []interface{}{"failed to close base64 encoder while encoding attestations", err}
		}
		return []interface{}{"attestations: ", buf.String()}
	})

	path, sourceEpochTimestamp := loader.MostRecentlyModified()
	c.tape.log.Debugf("using source epoch timestamp %s from most recently modified manifest file %q", sourceEpochTimestamp, path)
	packager := packager.NewDefaultPackager(client, c.OutputImage, &sourceEpochTimestamp, attreg.GetStatements()...)
	packageRef, err := packager.Push(ctx, images.Dir())
	if err != nil {
		return fmt.Errorf("failed to create package: %w", err)
	}

	c.tape.log.Infof("created package %q", packageRef)
	return nil
}
