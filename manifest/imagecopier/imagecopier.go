package imagecopier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"hash"

	"github.com/errordeveloper/tape/manifest/types"
	"github.com/errordeveloper/tape/oci"
)

type ImageCopier interface {
	CopyImages(context.Context, ...*types.ImageList) ([]string, error)
}

type RegistryCopier struct {
	*oci.Client

	DestinationRef string
	hash           hash.Hash
}

func NewRegistryCopier(client *oci.Client, destinationRef string) ImageCopier {
	if client == nil {
		client = oci.NewClient(nil)
	}
	return &RegistryCopier{
		Client:         client,
		DestinationRef: destinationRef,
		hash:           sha256.New(),
	}
}

func (c *RegistryCopier) CopyImages(ctx context.Context, lists ...*types.ImageList) ([]string, error) {
	copiedImages := []string{}
	for _, images := range lists {
		SetNewImageRefs(c.DestinationRef, c.hash, images.Items())
		for _, image := range images.Items() {
			newRef := image.NewName + ":" + image.NewTag
			if err := c.Copy(ctx, image.Ref(true), newRef, image.Digest); err != nil {
				return nil, err
			}
			copiedImages = append(copiedImages, newRef+"@"+image.Digest)
		}
	}
	return copiedImages, nil
}

func SetNewImageRefs(destinationRef string, hash hash.Hash, images []types.Image) {
	for i := range images {
		doSetNewImageRef(destinationRef, hash, &images[i])
	}
}

func doSetNewImageRef(destinationRef string, hash hash.Hash, i *types.Image) {
	i.NewName = destinationRef

	if oci.IsCosignArtifact(i.OriginalTag) {
		i.NewTag = i.OriginalTag // preserve tag of cosign artefact
		return
	}

	hash.Reset()
	_, _ = hash.Write([]byte(i.OriginalName + ":" + i.OriginalTag))
	i.NewTag = types.AppImageTagPrefix + hex.EncodeToString(hash.Sum(nil))
}
