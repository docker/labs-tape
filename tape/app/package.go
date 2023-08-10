package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	kimage "sigs.k8s.io/kustomize/api/image"

	"github.com/docker/labs-brown-tape/attest"
	"github.com/docker/labs-brown-tape/attest/manifest"
	"github.com/docker/labs-brown-tape/manifest/imagecopier"
	"github.com/docker/labs-brown-tape/manifest/imageresolver"
	"github.com/docker/labs-brown-tape/manifest/imagescanner"
	"github.com/docker/labs-brown-tape/manifest/loader"
	"github.com/docker/labs-brown-tape/manifest/packager"
	"github.com/docker/labs-brown-tape/manifest/updater"
	"github.com/docker/labs-brown-tape/oci"
)

type TapePackageCommand struct {
	CommonOptions

	OutputImage string `short:"O" long:"output-image" description:"Name of the output image" required:"true"`

	// TODO: implement
	Push bool `short:"P" long:"push" description:"Push the resulting image to the registry"`
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

	pathChecker, attreg, err := attest.DetectVCS(c.ManifestDir)
	if err != nil {
		return err
	}
	if pathChecker != nil {
		checked, _, err := pathChecker.Check()
		if err != nil {
			return err
		}
		if checked {
			summary, err := pathChecker.MakeSummary()
			if err != nil {
				return err
			}
			summaryJSON, err := json.Marshal(summary.Full())
			if err != nil {
				return err
			}
			c.tape.log.Infof("VCS info for %q: %s", c.ManifestDir, summaryJSON)
		} else {
			c.tape.log.Warnf("path %q is not in VCS", c.ManifestDir)
		}
	}

	scanner := imagescanner.NewDefaultImageScanner()
	scanner.WithProvinanceAttestor(attreg)

	if err := scanner.Scan(loader.RelPaths()); err != nil {
		return fmt.Errorf("failed to scan images: %w", err)
	}

	images := scanner.GetImages()
	c.tape.log.Debugf("found images: %#v", images.Items())

	attreg.AssociateStatements(manifest.MakeOriginalImageRefStatements(images)...)

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

	attreg.AssociateStatements(manifest.MakeResovedImageRefStatements(images)...)

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

	if err := attreg.EncodeAll(os.Stderr); err != nil {
		return err
	}

	path, sourceEpochTimestamp := loader.MostRecentlyModified()
	c.tape.log.Debugf("using source epoch timestamp %s from most recently modified manifest file %q", sourceEpochTimestamp, path)
	packager := packager.NewDefaultPackager(client, c.OutputImage, &sourceEpochTimestamp)
	packageRef, err := packager.Push(ctx, images.Dir())
	if err != nil {
		return fmt.Errorf("failed to create package: %w", err)
	}

	c.tape.log.Infof("created package %q", packageRef)
	return nil
}
