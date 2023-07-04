package imagecopier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"

	ociclient "github.com/fluxcd/pkg/oci/client"
	"github.com/google/go-containerregistry/pkg/crane"

	"github.com/docker/labs-brown-tape/manifest/types"
)

type ImageCopier interface {
	CopyImages(context.Context, *types.ImageList) error
}

type RegistryCopier struct {
	*ociclient.Client

	DestinationRef string
	hash           hash.Hash
}

func NewRegistryCopier(client *ociclient.Client, destinationRef string) ImageCopier {
	return &RegistryCopier{
		Client:         client,
		DestinationRef: destinationRef,
		hash:           sha256.New(),
	}
}

func (c *RegistryCopier) CopyImages(ctx context.Context, images *types.ImageList) error {
	options := append([]crane.Option{crane.WithContext(ctx)}, c.GetOptions()...)

	SetNewImageRefs(c.DestinationRef, c.hash, images.Items())
	for _, image := range images.Items() {
		newRef := image.NewName + ":" + image.NewTag
		if err := crane.Copy(image.OriginalRef, newRef, options...); err != nil {
			return err
		}
		digest, err := crane.Digest(newRef, options...)
		if err != nil {
			return err
		}
		if digest != image.Digest {
			return fmt.Errorf("unexpected digest mismatch after copying: %s (from destination registry) != %s (from source registry)", digest, image.Digest)
		}
	}
	return nil
}

func SetNewImageRefs(destinationRef string, hash hash.Hash, images []types.Image) {
	for i := range images {
		doSetNewImageRef(destinationRef, hash, &images[i])
	}
}

func doSetNewImageRef(destinationRef string, hash hash.Hash, i *types.Image) {
	i.NewName = destinationRef

	hash.Reset()
	_, _ = hash.Write([]byte(i.OriginalName + ":" + i.OriginalTag))
	i.NewTag = types.AppImageTagPrefix + hex.EncodeToString(hash.Sum(nil))
}
