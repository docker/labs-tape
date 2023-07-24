package oci

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	ociclient "github.com/fluxcd/pkg/oci/client"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	typesv1 "github.com/google/go-containerregistry/pkg/v1/types"
	// OCIv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type (
	Metadata   = ociclient.Metadata
	MediaType  = typesv1.MediaType
	Platform   = v1.Platform
	Hash       = v1.Hash
	Descriptor = v1.Descriptor
	Client     struct {
		*ociclient.Client
	}
)

func NewClient(opts []crane.Option) *Client {
	return &Client{Client: ociclient.NewClient(opts)}
}

func (c *Client) withContext(ctx context.Context) []crane.Option {
	return append([]crane.Option{
		crane.WithContext(ctx),
	}, c.GetOptions()...)
}

func (c *Client) Digest(ctx context.Context, ref string) (string, error) {
	return crane.Digest(ref, c.withContext(ctx)...)
}

func (c *Client) Copy(ctx context.Context, srcRef, dstRef, digest string) error {
	if err := crane.Copy(srcRef, dstRef, c.withContext(ctx)...); err != nil {
		return err
	}
	newDigest, err := crane.Digest(dstRef, c.withContext(ctx)...)
	if err != nil {
		return err
	}
	if digest != newDigest {
		return fmt.Errorf("unexpected digest mismatch after copying: %s (from destination registry) != %s (from source registry)", digest, newDigest)
	}
	return nil
}

func (c *Client) Index(ctx context.Context, ref string) (*v1.IndexManifest, error) {
	data, err := crane.Manifest(ref, c.withContext(ctx)...)
	if err != nil {
		return nil, err
	}

	return v1.ParseIndexManifest(bytes.NewReader(data))
}

func (c *Client) PullArtefact(ctx context.Context, ref, dir string) (*Metadata, error) {
	return c.Client.Pull(ctx, ref, dir)
}

func (c *Client) Pull(ctx context.Context, ref string) (v1.Image, error) {
	return crane.Pull(ref, c.withContext(ctx)...)
}

func (c *Client) ListRelated(ctx context.Context, ref, digest string) ([]Metadata, error) {
	tagPrefix := strings.Join(strings.Split(digest, ":"), "-")
	listOptions := ociclient.ListOptions{
		RegexFilter:            fmt.Sprintf("^%s.*", tagPrefix),
		IncludeCosignArtifacts: true,
	}
	tags, err := c.List(ctx, ref, listOptions)
	if err != nil {
		return nil, err
	}
	return tags, nil
}

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

func IsCosignArtifact(ref string) bool {
	return ociclient.IsCosignArtifact(ref)
}
