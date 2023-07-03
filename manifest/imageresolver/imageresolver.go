package imageresolver

import (
	"context"
	"fmt"

	ociclient "github.com/fluxcd/pkg/oci/client"
	"github.com/google/go-containerregistry/pkg/crane"

	"github.com/docker/labs-brown-tape/manifest/types"
)

type Resolver interface {
	ResolveDigests(context.Context, *types.ImageList) error
}

// TODO: add known digests to RegistryResolver, so that user can specify digests of newly built images
type RegistryResolver struct {
	*ociclient.Client
}

func NewRegistryResolver(client *ociclient.Client) Resolver {
	return &RegistryResolver{
		Client: client,
	}
}

func (r *RegistryResolver) ResolveDigests(ctx context.Context, images *types.ImageList) error {
	for i := range images.Items() {
		if err := r.doResolveDigest(ctx, &images.Items()[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *RegistryResolver) doResolveDigest(ctx context.Context, i *types.Image) error {
	options := append([]crane.Option{crane.WithContext(ctx)}, r.GetOptions()...)

	digest, err := crane.Digest(i.OriginalRef, options...)
	if err != nil {
		return err
	}
	if i.Digest != "" && i.Digest != digest {
		// this is unexpected as when digest is being specified, that is exactly what will be retrieved
		return fmt.Errorf("unexpected digest mismatch: %s (from manifest) != %s (form registry)", i.Digest, digest)
	}
	i.Digest = digest
	return nil
}

// type FakeResolver struct{}

// func NewFakeResolver() Resolver {
// 	return &FakeResolver{}
// }

// func (r *FakeResolver) ResolveDigest(i *types.Image) error {
// 	if i.Digest != "" {
// 		return nil
// 	}

// 	i.Digest = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // empty string
// 	return nil
// }
