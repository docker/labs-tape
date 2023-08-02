package oci

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	ociclient "github.com/fluxcd/pkg/oci"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	typesv1 "github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/docker/labs-brown-tape/manifest/types"
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
func (c *Client) PushArtefact(ctx context.Context, destinationRef, sourceDir string) (string, error) {

	tmpDir, err := os.MkdirTemp("", "bpt-oci-artefact-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "artefact.tgz")

	if err := c.Build(tmpFile, sourceDir, nil); err != nil {
		return "", err
	}

	// TODO: can avoid re-reading the file by rewrtiting the Build function and passing io.TeeWriter
	data, err := os.OpenFile(tmpFile, os.O_RDONLY, 0)
	if err != nil {
		return "", err
	}

	c.hash.Reset()
	if _, err := io.Copy(c.hash, data); err != nil {
		return "", err
	}

	ref := destinationRef + ":" + types.ConfigImageTagPrefix + hex.EncodeToString(c.hash.Sum(nil))
	// _, err := name.ParseReference(ref)
	// if err != nil {
	// 	return "", fmt.Errorf("invalid URL: %w", err)
	// }

	// if meta.Created == "" {
	// 	ct := time.Now().UTC()
	// 	meta.Created = ct.Format(time.RFC3339)
	// }

	// TODO: define tape media types
	img := mutate.MediaType(empty.Image, typesv1.OCIManifestSchema1)
	img = mutate.ConfigMediaType(img, ociclient.CanonicalConfigMediaType)
	// img = mutate.Annotations(img, meta.ToAnnotations()).(v1.Image)

	layer, err := tarball.LayerFromFile(tmpFile, tarball.WithMediaType(ociclient.CanonicalContentMediaType))
	if err != nil {
		return "", fmt.Errorf("creating content layer failed: %w", err)
	}

	img, err = mutate.Append(img, mutate.Addendum{Layer: layer})
	if err != nil {
		return "", fmt.Errorf("appeding content to artifact failed: %w", err)
	}

	if err := crane.Push(img, ref, c.artefactPushOptions(ctx)...); err != nil {
		return "", fmt.Errorf("pushing artifact failed: %w", err)
	}

	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("parsing artifact digest failed: %w", err)
	}

	return ref + "@" + digest.String(), err
}

func (c *Client) artefactPushOptions(ctx context.Context) []crane.Option {
	return append(c.withContext(ctx),
		crane.WithPlatform(&v1.Platform{
			Architecture: "unknown",
			OS:           "unknown",
		}))
}
