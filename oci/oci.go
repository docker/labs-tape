package oci

import (
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"strings"

	ociclient "github.com/fluxcd/pkg/oci/client"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/types"
	typesv1 "github.com/google/go-containerregistry/pkg/v1/types"

	// OCIv1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/google/go-containerregistry/pkg/logs"
)

const (
	UserAgent = "tape/v1"
)

type (
	Descriptor    = v1.Descriptor
	Hash          = v1.Hash
	Image         = v1.Image
	ImageIndex    = v1.ImageIndex
	IndexManifest = v1.IndexManifest
	Layer         = v1.Layer
	Manifest      = v1.Manifest
	MediaType     = typesv1.MediaType
	Metadata      = ociclient.Metadata
	Platform      = v1.Platform
	Client        struct {
		*ociclient.Client
		hash hash.Hash
	}
)

func NewClient(opts []crane.Option) *Client {
	options := []crane.Option{
		crane.WithUserAgent(UserAgent),
	}
	options = append(options, opts...)

	return &Client{
		Client: ociclient.NewClient(options),
		hash:   sha256.New(),
	}
}

func NewDebugClient(debugWriter io.Writer, opts []crane.Option) *Client {
	logs.Debug.SetOutput(debugWriter)

	return NewClient([]crane.Option{
		crane.WithTransport(transport.NewLogger(remote.DefaultTransport)),
	})
}

func (c *Client) withContext(ctx context.Context) []crane.Option {
	return append([]crane.Option{
		crane.WithContext(ctx),
	}, c.GetOptions()...)
}

func (c *Client) remoteWithContext(ctx context.Context) []remote.Option {
	return append([]remote.Option{
		remote.WithContext(ctx),
	}, crane.GetOptions(c.GetOptions()...).Remote...)
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

func (c *Client) GetIndexOrImage(ctx context.Context, ref string) (v1.ImageIndex, *v1.IndexManifest, v1.Image, error) {
	parsedRef, err := name.ParseReference(ref)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid URL %q: %w", ref, err)
	}

	var imageIndex v1.ImageIndex

	head, err := remote.Head(parsedRef, c.remoteWithContext(ctx)...)
	if err != nil {
		return nil, nil, nil, err
	}
	switch head.MediaType {
	case typesv1.OCIImageIndex, types.DockerManifestList:
		imageIndex, err = remote.Index(parsedRef, c.remoteWithContext(ctx)...)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get index for %s: %w", ref, err)
		}

		indexManifest, err := imageIndex.IndexManifest()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get index manifest for %s: %w", ref, err)
		}

		if len(indexManifest.Manifests) == 0 {
			return nil, nil, nil, fmt.Errorf("no manifests found in image %q", ref)
		}

		return imageIndex, indexManifest, nil, nil
	default:
		descriptor, err := remote.Get(parsedRef, c.remoteWithContext(ctx)...)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get descriptor for %s: %w", ref, err)
		}

		image, err := descriptor.Image()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get image index for %s: %w", ref, err)
		}

		return nil, nil, image, nil
	}
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

func IsCosignArtifact(ref string) bool {
	return ociclient.IsCosignArtifact(ref)
}
