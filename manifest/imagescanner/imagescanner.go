package imagescanner

import (
	"encoding/hex"
	"hash"
	"io"
	"os"
	"path/filepath"

	"crypto/sha256"

	kimage "sigs.k8s.io/kustomize/api/image"
	"sigs.k8s.io/kustomize/kyaml/kio"

	"github.com/docker/labs-brown-tape/manifest/types"
)

// TODO(attest): provide an attestation for each original file inspected and the image refs found
type ImageScanner interface {
	Scan(string, []string) error
	GetImages() *types.ImageList
	Reset()
}

type DefaultImageScanner struct {
	directory string
	trackers  []*Tracker
	hash      hash.Hash
}

func NewDefaultImageScanner() ImageScanner {
	return &DefaultImageScanner{
		trackers: []*Tracker{},
		hash:     sha256.New(),
	}
}

func (s *DefaultImageScanner) Scan(dir string, manifests []string) error {
	s.directory = dir
	for m := range manifests {
		manifest, err := os.Open(filepath.Join(dir, manifests[m]))
		if err != nil {
			return err
		}

		s.hash.Reset()

		filter := &Filter{}
		tracker := &Tracker{
			Manifest: manifests[m],
		}

		filter.WithMutationTracker(tracker.MutationTracker)

		pipeline := kio.Pipeline{
			Inputs: []kio.Reader{
				&kio.ByteReader{Reader: io.TeeReader(manifest, s.hash)},
			},
			Filters: []kio.Filter{filter},
		}

		if err := pipeline.Execute(); err != nil {
			return err
		}

		tracker.ManifestDigest = hex.EncodeToString(s.hash.Sum(nil))
		s.trackers = append(s.trackers, tracker)
	}
	return nil
}

func (s *DefaultImageScanner) GetImages() *types.ImageList {
	images := types.NewImageList(s.directory)
	for _, v := range s.trackers {
		for _, vv := range v.SetValueArgs() {
			name, tag, digest := kimage.Split(vv.Value)
			images.Append(types.Image{
				Source: &types.Source{
					ImageSourceLocation: types.ImageSourceLocation{
						Manifest:       v.Manifest,
						ManifestDigest: v.ManifestDigest,
						NodePath:       vv.NodePath,
						Line:           vv.Line,
						Column:         vv.Column,
					},
					OriginalRef: vv.Value,
				},
				OriginalName: name,
				OriginalTag:  tag,
				Digest:       digest,
			})
		}
	}
	return images
}

func (s *DefaultImageScanner) Reset() {
	s.trackers = []*Tracker{}
}
