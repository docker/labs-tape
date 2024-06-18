package oci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"time"

	ociclient "github.com/fluxcd/pkg/oci"
	"github.com/go-git/go-git/v5/utils/ioutil"
	"github.com/google/go-containerregistry/pkg/compression"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	typesv1 "github.com/google/go-containerregistry/pkg/v1/types"

	attestTypes "github.com/errordeveloper/tape/attest/types"
	manifestTypes "github.com/errordeveloper/tape/manifest/types"
)

const (
	mediaTypePrefix            = "application/vnd.docker.tape"
	ConfigMediaType  MediaType = mediaTypePrefix + ".config.v1alpha1+json"
	ContentMediaType MediaType = mediaTypePrefix + ".content.v1alpha1.tar+gzip"
	AttestMediaType  MediaType = mediaTypePrefix + ".attest.v1alpha1.jsonl+gzip"

	ContentInterpreterAnnotation   = mediaTypePrefix + ".content-interpreter.v1alpha1"
	ContentInterpreterKubectlApply = mediaTypePrefix + ".kubectl-apply.v1alpha1.tar+gzip"

	AttestationsSummaryAnnotation = mediaTypePrefix + ".attestations-summary.v1alpha1"

	// TODO: content interpreter invocation with an image

	regularFileMode = 0o640

	OCIManifestSchema1 = typesv1.OCIManifestSchema1
)

type ArtefactInfo struct {
	io.ReadCloser

	MediaType   MediaType
	Annotations map[string]string
	Digest      string
}

func (c *Client) Fetch(ctx context.Context, ref string, mediaTypes ...MediaType) ([]*ArtefactInfo, error) {
	imageIndex, indexManifest, image, err := c.GetIndexOrImage(ctx, ref)
	if err != nil {
		return nil, err
	}
	artefactInfo, _, err := c.FetchFromIndexOrImage(ctx, imageIndex, indexManifest, image, mediaTypes...)
	return artefactInfo, err
}

func (c *Client) FetchFromIndexOrImage(ctx context.Context, imageIndex ImageIndex, indexManifest *IndexManifest, image Image, mediaTypes ...MediaType) ([]*ArtefactInfo, map[Hash]*Manifest, error) {
	numMediaTypes := len(mediaTypes)

	selector := make(map[MediaType]struct{}, numMediaTypes)
	for _, mediaType := range mediaTypes {
		selector[mediaType] = struct{}{}
	}

	skip := func(mediaType MediaType) bool {
		if numMediaTypes > 0 {
			if _, ok := selector[mediaType]; !ok {
				return true
			}
		}
		return false
	}

	artefacts := []*ArtefactInfo{}
	manifests := map[Hash]*Manifest{}

	if indexManifest == nil {
		manifest, err := image.Manifest()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get manifest: %w", err)
		}

		imageDigest, err := image.Digest()
		if err != nil {
			return nil, nil, err
		}
		manifests[imageDigest] = manifest

		for j := range manifest.Layers {
			layerDescriptor := manifest.Layers[j]

			if skip(layerDescriptor.MediaType) {
				continue
			}

			info, err := newArtifcatInfoFromLayerDescriptor(image, layerDescriptor, manifest.Annotations)
			if err != nil {
				return nil, nil, err
			}
			artefacts = append(artefacts, info)
		}
		return artefacts, manifests, nil
	}

	for i := range indexManifest.Manifests {
		manifestDescriptor := indexManifest.Manifests[i]

		if skip(MediaType(manifestDescriptor.ArtifactType)) {
			continue
		}

		image, manifest, err := c.getImage(ctx, imageIndex, manifestDescriptor.Digest)
		if err != nil {
			return nil, nil, err
		}

		manifests[manifestDescriptor.Digest] = manifest

		for j := range manifest.Layers {
			layerDescriptor := manifest.Layers[j]

			if layerDescriptor.MediaType != MediaType(manifestDescriptor.ArtifactType) {
				return nil, nil, fmt.Errorf("media type mismatch between manifest and layer: %s != %s", manifestDescriptor.MediaType, layerDescriptor.MediaType)
			}

			info, err := newArtifcatInfoFromLayerDescriptor(image, layerDescriptor, manifest.Annotations)
			if err != nil {
				return nil, nil, err
			}
			artefacts = append(artefacts, info)
		}
	}

	return artefacts, manifests, nil
}

func (c *Client) GetSingleArtefact(ctx context.Context, ref string) (*ArtefactInfo, error) {
	image, layers, annotations, err := c.getFlatArtefactLayers(ctx, ref)
	if err != nil {
		return nil, err
	}
	if len(layers) != 1 {
		return nil, fmt.Errorf("multiple layers found in image %q", ref)
	}
	return newArtifcatInfoFromLayerDescriptor(image, layers[0], annotations)
}

func newArtifcatInfoFromLayerDescriptor(image Image, layerDecriptor Descriptor, annotations map[string]string) (*ArtefactInfo, error) {
	layer, err := image.LayerByDigest(layerDecriptor.Digest)
	if err != nil {
		return nil, fmt.Errorf("fetching artefact image failed: %w", err)
	}

	blob, err := layer.Compressed()
	if err != nil {
		return nil, fmt.Errorf("extracting compressed aretefact image failed: %w", err)
	}
	info := &ArtefactInfo{
		ReadCloser:  blob,
		MediaType:   layerDecriptor.MediaType,
		Annotations: annotations,
		Digest:      layerDecriptor.Digest.String(),
	}
	return info, nil
}

func (c *Client) getFlatArtefactLayers(ctx context.Context, ref string) (Image, []Descriptor, map[string]string, error) {
	imageIndex, indexManifest, image, err := c.GetIndexOrImage(ctx, ref)
	if err != nil {
		return nil, nil, nil, err
	}

	var manifest *Manifest

	if indexManifest != nil {
		if len(indexManifest.Manifests) != 1 {
			return nil, nil, nil, fmt.Errorf("multiple manifests found in image %q", ref)
		}

		image, manifest, err = c.getImage(ctx, imageIndex, indexManifest.Manifests[0].Digest)
		if err != nil {
			return nil, nil, nil, err
		}
	} else {
		manifest, err = image.Manifest()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get manifest for %q: %w", ref, err)
		}
	}

	if len(manifest.Layers) < 1 {
		return nil, nil, nil, fmt.Errorf("no layers found in image %q", ref)
	}

	return image, manifest.Layers, manifest.Annotations, nil
}

func (c *Client) getImage(ctx context.Context, imageIndex ImageIndex, digest Hash) (Image, *Manifest, error) {
	image, err := imageIndex.Image(digest)
	if err != nil {
		return nil, nil, err
	}
	manifest, err := image.Manifest()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get manifest for %q: %w", digest.String(), err)
	}
	return image, manifest, nil
}

// based on https://github.com/fluxcd/pkg/blob/2a323d771e17af02dee2ccbbb9b445b78ab048e5/oci/client/push.go
func (c *Client) PushArtefact(ctx context.Context, destinationRef, sourceDir string, timestamp *time.Time, sourceAttestations ...attestTypes.Statement) (string, error) {
	tmpDir, err := os.MkdirTemp("", "bpt-oci-artefact-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "artefact.tgz")

	outputFile, err := os.OpenFile(tmpFile, os.O_RDWR|os.O_CREATE|os.O_EXCL, regularFileMode)
	if err != nil {
		return "", err
	}
	defer outputFile.Close()

	c.hash.Reset()

	output := io.MultiWriter(outputFile, c.hash)

	if err := c.BuildArtefact(tmpFile, sourceDir, output); err != nil {
		return "", err
	}

	attestLayer, err := c.BuildAttestations(sourceAttestations)
	if err != nil {
		return "", fmt.Errorf("failed to serialise attestations: %w", err)
	}

	repo, err := name.NewRepository(destinationRef)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	hash := hex.EncodeToString(c.hash.Sum(nil))
	tag := repo.Tag(manifestTypes.ConfigImageTagPrefix + hash)
	tagAlias := tag.Context().Tag(manifestTypes.ConfigImageTagPrefix + hash[:7])

	if timestamp == nil {
		timestamp = new(time.Time)
		*timestamp = time.Now().UTC()
	}

	indexAnnotations := map[string]string{
		ociclient.CreatedAnnotation: timestamp.Format(time.RFC3339),
	}

	index := mutate.Annotations(
		empty.Index,
		indexAnnotations,
	).(ImageIndex)

	configAnnotations := maps.Clone(indexAnnotations)

	configAnnotations[ContentInterpreterAnnotation] = ContentInterpreterKubectlApply

	config := mutate.Annotations(
		mutate.ConfigMediaType(
			mutate.MediaType(empty.Image, OCIManifestSchema1),
			ContentMediaType,
		),
		configAnnotations,
	).(Image)

	// There is an option to use LayerFromReader which will avoid writing any files to disk,
	// albeit it might impact memory usage and there is no strict security requirement, and
	// manifests do get written out already anyway.
	configLayer, err := tarball.LayerFromFile(tmpFile,
		tarball.WithMediaType(ContentMediaType),
		tarball.WithCompression(compression.GZip),
		tarball.WithCompressedCaching,
	)
	if err != nil {
		return "", fmt.Errorf("creating artefact content layer failed: %w", err)
	}

	config, err = mutate.Append(config, mutate.Addendum{Layer: configLayer})
	if err != nil {
		return "", fmt.Errorf("appeding content to artifact failed: %w", err)
	}

	index = mutate.AppendManifests(index,
		mutate.IndexAddendum{
			Descriptor: makeDescriptorWithPlatform(),
			Add:        config,
		},
	)

	if attestLayer != nil {
		attestAnnotations := maps.Clone(indexAnnotations)

		summary, err := (attestTypes.Statements)(sourceAttestations).MarshalSummaryAnnotation()
		if err != nil {
			return "", err
		}
		attestAnnotations[AttestationsSummaryAnnotation] = summary

		attest := mutate.Annotations(
			mutate.ConfigMediaType(
				mutate.MediaType(empty.Image, OCIManifestSchema1),
				AttestMediaType,
			),
			attestAnnotations,
		).(Image)

		attest, err = mutate.Append(attest, mutate.Addendum{Layer: attestLayer})
		if err != nil {
			return "", fmt.Errorf("appeding attestations to artifact failed: %w", err)
		}

		index = mutate.AppendManifests(index,
			mutate.IndexAddendum{
				Descriptor: makeDescriptorWithPlatform(),
				Add:        attest,
			},
		)
	}

	digest, err := index.Digest()
	if err != nil {
		return "", fmt.Errorf("parsing index digest failed: %w", err)
	}

	if err := remote.WriteIndex(tag, index, c.remoteWithContext(ctx)...); err != nil {
		return "", fmt.Errorf("pushing index failed: %w", err)
	}

	if err := remote.Tag(tagAlias, index, c.remoteWithContext(ctx)...); err != nil {
		return "", fmt.Errorf("adding alias tagging failed: %w", err)
	}

	return tagAlias.String() + "@" + digest.String(), err
}

func makeDescriptorWithPlatform() Descriptor {
	return Descriptor{
		Platform: &Platform{
			Architecture: "unknown",
			OS:           "unknown",
		},
	}
}

// based on https://github.com/fluxcd/pkg/blob/2a323d771e17af02dee2ccbbb9b445b78ab048e5/oci/client/build.go
func (c *Client) BuildArtefact(artifactPath,
	sourceDir string, output io.Writer) error {
	absDir, err := filepath.Abs(sourceDir)
	if err != nil {
		return err
	}

	dirStat, err := os.Stat(absDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("invalid source dir path: %s", absDir)
	}

	gw := gzip.NewWriter(output)
	tw := tar.NewWriter(gw)
	if err := filepath.WalkDir(absDir, func(p string, di os.DirEntry, prevErr error) (err error) {
		if prevErr != nil {
			return prevErr
		}

		// Ignore anything that is not a file or directories e.g. symlinks
		ft := di.Type()
		if !(ft.IsRegular() || ft.IsDir()) {
			return nil
		}

		fi, err := di.Info()
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, p)
		if err != nil {
			return err
		}
		if dirStat.IsDir() {
			// The name needs to be modified to maintain directory structure
			// as tar.FileInfoHeader only has access to the base name of the file.
			// Ref: https://golang.org/src/archive/tar/common.go?#L6264
			//
			// we only want to do this if a directory was passed in
			relFilePath, err := filepath.Rel(absDir, p)
			if err != nil {
				return err
			}
			// Normalize file path so it works on windows
			header.Name = filepath.ToSlash(relFilePath)
		}

		// Remove any environment specific data.
		header.Gid = 0
		header.Uid = 0
		header.Uname = ""
		header.Gname = ""
		header.ModTime = time.Time{}
		header.AccessTime = time.Time{}
		header.ChangeTime = time.Time{}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !ft.IsRegular() {
			return nil
		}

		file, err := os.Open(p)
		if err != nil {
			return err
		}
		defer ioutil.CheckClose(file, &err)

		if _, err := io.Copy(tw, file); err != nil {
			return err
		}
		return nil
	}); err != nil {
		_ = tw.Close()
		_ = gw.Close()
		return err
	}

	if err := tw.Close(); err != nil {
		_ = gw.Close()
		return err
	}
	if err := gw.Close(); err != nil {
		return err
	}

	return nil
}

func (c *Client) BuildAttestations(statements []attestTypes.Statement) (Layer, error) {
	if len(statements) == 0 {
		return nil, nil
	}
	output := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(output)

	if err := attestTypes.Statements(statements).Encode(gw); err != nil {
		return nil, err
	}

	if err := gw.Close(); err != nil {
		return nil, err
	}

	layer, err := tarball.LayerFromOpener(
		func() (io.ReadCloser, error) {
			// this doesn't copy data, it should re-use same undelying slice
			return io.NopCloser(bytes.NewReader(output.Bytes())), nil
		},
		tarball.WithMediaType(AttestMediaType),
		tarball.WithCompression(compression.GZip),
		tarball.WithCompressedCaching,
	)
	if err != nil {
		return nil, fmt.Errorf("creating attestations layer failed: %w", err)
	}

	return layer, nil
}
