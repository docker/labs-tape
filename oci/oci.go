package oci

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"strings"

	ociclient "github.com/fluxcd/pkg/oci/client"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	typesv1 "github.com/google/go-containerregistry/pkg/v1/types"
	// OCIv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

const UserAgent = "tape/v1"

type (
	Metadata   = ociclient.Metadata
	MediaType  = typesv1.MediaType
	Platform   = v1.Platform
	Hash       = v1.Hash
	Descriptor = v1.Descriptor
	Client     struct {
		*ociclient.Client
		hash hash.Hash
	}
)

func NewClient(opts []crane.Option) *Client {
	options := []crane.Option{
		crane.WithUserAgent(UserAgent),
		crane.WithPlatform(&v1.Platform{
			Architecture: "unknown",
			OS:           "unknown",
		}),
	}
	options = append(options, opts...)

	return &Client{
		Client: ociclient.NewClient(options),
		hash:   sha256.New(),
	}
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

func IsCosignArtifact(ref string) bool {
	return ociclient.IsCosignArtifact(ref)
}
