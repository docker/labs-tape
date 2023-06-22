package imagescanner

import (
	"encoding/hex"
	"io"
	"os"

	"crypto/sha256"

	"sigs.k8s.io/kustomize/api/image"
	"sigs.k8s.io/kustomize/kyaml/kio"

	"github.com/docker/labs-brown-tape/manifest/types"
)

// TODO(attest): provide an attestation for each original file inspected and the image refs found

type ImageScanner struct {
	trackers []*Tracker
}

func NewImageScanner() *ImageScanner {
	return &ImageScanner{
		trackers: []*Tracker{},
	}
}

func (s *ImageScanner) Scan(manifests []string) error {
	hash := sha256.New()

	for m := range manifests {
		manifest, err := os.Open(manifests[m])
		if err != nil {
			return err
		}

		hash.Reset()

		filter := &Filter{}
		tracker := &Tracker{
			Manifest: manifests[m],
		}

		filter.WithMutationTracker(tracker.MutationTracker)

		pipeline := kio.Pipeline{
			Inputs: []kio.Reader{
				&kio.ByteReader{Reader: io.TeeReader(manifest, hash)},
			},
			Filters: []kio.Filter{filter},
		}

		if err := pipeline.Execute(); err != nil {
			return err
		}

		tracker.ManifestDigest = hex.EncodeToString(hash.Sum(nil))
		s.trackers = append(s.trackers, tracker)
	}
	return nil
}

func (i *ImageScanner) GetImages() []types.Image {
	images := []types.Image{}
	for _, v := range i.trackers {
		for _, vv := range v.SetValueArgs() {

			name, tag, digest := image.Split(vv.Value)
			images = append(images, types.Image{
				Manifest:       v.Manifest,
				ManifestDigest: v.ManifestDigest,
				NodePath:       vv.NodePath,
				OriginalRef:    vv.Value,
				OriginalName:   name,
				OriginalTag:    tag,
				Digest:         digest,
			})
		}
	}
	i.trackers = []*Tracker{}
	return images
}

func (i *ImageScanner) Reset() {
	i.trackers = []*Tracker{}
}
