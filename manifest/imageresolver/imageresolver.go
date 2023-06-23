package imageresolver

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"

	"github.com/docker/labs-brown-tape/manifest/types"
)

type Resolver interface {
	ResolveDigests([]types.Image) error
}

type RegistryResolver struct{}

func NewRegistryResolver() Resolver {
	return &RegistryResolver{}
}

func (r *RegistryResolver) ResolveDigests(images []types.Image) error {
	for i := range images {
		if err := r.doResolveDigest(&images[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *RegistryResolver) doResolveDigest(i *types.Image) error {
	digest, err := crane.Digest(i.OriginalRef, crane.WithAuthFromKeychain(authn.DefaultKeychain))
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
