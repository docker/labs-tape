package imagecopier

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"

	"github.com/docker/labs-brown-tape/manifest/types"
)

type ImageCopier interface {
	CopyImages([]types.Image) error
}

type RegistryCopier struct {
	DestinationRef string
	hash           hash.Hash
}

func NewRegistryCopier(destinationRef string) ImageCopier {
	return &RegistryCopier{
		DestinationRef: destinationRef,
		hash:           sha256.New(),
	}
}

func (c *RegistryCopier) CopyImages(images []types.Image) error {
	setNewImageRefs(c.DestinationRef, c.hash, images)
	for _, image := range images {
		if err := crane.Copy(image.OriginalRef, image.NewRef, crane.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
			return err
		}
		digest, err := crane.Digest(image.NewRef, crane.WithAuthFromKeychain(authn.DefaultKeychain))
		if err != nil {
			return err
		}
		if digest != image.Digest {
			return fmt.Errorf("unexpected digest mismatch after copying: %s (from destination registry) != %s (from source registry)", digest, image.Digest)
		}
	}
	return nil
}

func setNewImageRefs(destinationRef string, hash hash.Hash, images []types.Image) {
	for i := range images {
		doSetNewImageRef(destinationRef, hash, &images[i])
	}
}

func doSetNewImageRef(destinationRef string, hash hash.Hash, i *types.Image) {
	hash.Reset()
	hash.Write([]byte(i.OriginalName + ":" + i.OriginalTag))
	tag := hex.EncodeToString(hash.Sum(nil))

	i.NewRef = destinationRef + ":" + tag + "@" + i.Digest
}
