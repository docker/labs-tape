package oci

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/fluxcd/pkg/oci"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type ArtefactInfo struct {
	io.ReadCloser

	MediaType   MediaType
	Annotations map[string]string
}

func (c *Client) GetArtefact(ctx context.Context, ref string) (*ArtefactInfo, error) {
	image, err := c.Pull(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to pull %q: %w", ref, err)
	}
	manifest, err := image.Manifest()
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest of %q: %w", ref, err)
	}
	if len(manifest.Layers) < 1 {
		return nil, fmt.Errorf("no layers found in image %q", ref)
	}
	if len(manifest.Layers) > 1 {
		return nil, fmt.Errorf("multiple layers found in image %q", ref)
	}
	layerDecriptor := manifest.Layers[0]

	layer, err := image.LayerByDigest(layerDecriptor.Digest)
	if err != nil {
		return nil, fmt.Errorf("fetching aretefact image failed: %w", err)
	}

	blob, err := layer.Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("extracting uncompressed aretefact image failed: %w", err)
	}

	info := &ArtefactInfo{
		ReadCloser:  blob,
		MediaType:   layerDecriptor.MediaType,
		Annotations: layerDecriptor.Annotations,
	}

	return info, nil
}

// based on https://github.com/fluxcd/pkg/blob/2a323d771e17af02dee2ccbbb9b445b78ab048e5/oci/client/push.go
func (c *Client) PushArtefact(ctx context.Context, url, sourceDir string, meta Metadata, ignorePaths []string) (string, error) {
	ref, err := name.ParseReference(url)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "oci")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "artifact.tgz")

	if err := c.Build(tmpFile, sourceDir, ignorePaths); err != nil {
		return "", err
	}

	if meta.Created == "" {
		ct := time.Now().UTC()
		meta.Created = ct.Format(time.RFC3339)
	}

	img := mutate.MediaType(empty.Image, types.OCIManifestSchema1)
	img = mutate.ConfigMediaType(img, oci.CanonicalConfigMediaType)
	img = mutate.Annotations(img, meta.ToAnnotations()).(gcrv1.Image)

	layer, err := tarball.LayerFromFile(tmpFile, tarball.WithMediaType(oci.CanonicalContentMediaType))
	if err != nil {
		return "", fmt.Errorf("creating content layer failed: %w", err)
	}

	img, err = mutate.Append(img, mutate.Addendum{Layer: layer})
	if err != nil {
		return "", fmt.Errorf("appeding content to artifact failed: %w", err)
	}

	if err := crane.Push(img, url, c.optionsWithContext(ctx)...); err != nil {
		return "", fmt.Errorf("pushing artifact failed: %w", err)
	}

	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("parsing artifact digest failed: %w", err)
	}

	return ref.Context().Digest(digest.String()).String(), err
}
