package imageresolver

import (
	"context"
	"fmt"

	kimage "sigs.k8s.io/kustomize/api/image"

	"github.com/docker/labs-brown-tape/manifest/types"
	"github.com/docker/labs-brown-tape/oci"
)

type Resolver interface {
	ResolveDigests(context.Context, *types.ImageList) error
	FindRelatedTags(context.Context, *types.ImageList) (*types.ImageList, error)
	FindRelatedFromIndecies(context.Context, *types.ImageList, func(*types.Image, oci.Descriptor) error) (*types.ImageList, *types.ImageList, error)
}

// TODO: add known digests to RegistryResolver, so that user can specify digests of newly built images
type RegistryResolver struct {
	*oci.Client
}

func NewRegistryResolver(client *oci.Client) Resolver {
	if client == nil {
		client = oci.NewClient(nil)
	}
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
	digest, err := r.Digest(ctx, i.Ref(true))
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

func (c *RegistryResolver) FindRelatedTags(ctx context.Context, images *types.ImageList) (*types.ImageList, error) {
	// TODO: reduce redudant calls to registry, e.g. when multiple images have the same name
	result := types.NewImageList(images.Dir())
	for i := range images.Items() {
		image := images.Items()[i]
		if image.Digest == "" {
			return nil, fmt.Errorf("image %s has no digest", image.Ref(true))
		}
		related, err := c.ListRelated(ctx, image.OriginalName, image.Digest)
		if err != nil {
			return nil, fmt.Errorf("failed to list related tag for %s: %w", image.Ref(true), err)
		}
		for i := range related {
			relatedImage := &related[i]
			if relatedImage.Digest == "" {
				return nil, fmt.Errorf("related image %s has no digest", relatedImage.URL)
			}
			name, tag, _ := kimage.Split(relatedImage.URL)

			err := result.AppendWithRelationTo(image, types.Image{
				Source: &types.Source{
					OriginalRef: relatedImage.URL,
				},
				OriginalName: name,
				OriginalTag:  tag,
				Digest:       relatedImage.Digest,
			})
			if err != nil {
				return nil, err
			}

		}
	}
	if err := result.Dedup(); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *RegistryResolver) FindRelatedFromIndecies(ctx context.Context, images *types.ImageList, inspect func(*types.Image, oci.Descriptor) error) (*types.ImageList, *types.ImageList, error) {
	manifests := types.NewImageList(images.Dir())
	for i := range images.Items() {
		image := images.Items()[i]
		index, err := c.Index(ctx, image.Ref(true))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get index for %s: %w", image.Ref(true), err)
		}
		for i := range index.Manifests {
			manifest := index.Manifests[i]
			if inspect != nil {
				if err := inspect(&image, manifest); err != nil {
					return nil, nil, err
				}
			}
			err := manifests.AppendWithRelationTo(image, types.Image{
				Source: &types.Source{
					OriginalRef: image.OriginalName,
				},
				OriginalName: image.OriginalName,
				Digest:       manifest.Digest.String(),
			})
			if err != nil {
				return nil, nil, err
			}
		}
	}
	related, err := c.FindRelatedTags(ctx, manifests)
	if err != nil {
		return nil, nil, err
	}
	return manifests, related, nil
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
