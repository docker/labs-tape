package imagescanner

import (
	"hash"
	"io"
	"os"
	"path/filepath"

	"crypto/sha256"

	kimage "sigs.k8s.io/kustomize/api/image"
	"sigs.k8s.io/kustomize/kyaml/kio"

	"github.com/errordeveloper/tape/attest"
	"github.com/errordeveloper/tape/attest/digest"
	"github.com/errordeveloper/tape/manifest/types"
)

type ImageScanner interface {
	Scan(string, []string) error
	GetImages() *types.ImageList
	Reset()
	WithProvinanceAttestor(*attest.PathCheckerRegistry)
}

type DefaultImageScanner struct {
	directory string
	trackers  []*Tracker
	hash      hash.Hash
	attestor  *attest.PathCheckerRegistry
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
				&kio.ByteReader{
					Reader:                io.TeeReader(manifest, s.hash),
					OmitReaderAnnotations: true,
				},
			},
			Filters: []kio.Filter{filter},
		}

		if err := pipeline.Execute(); err != nil {
			return err
		}

		tracker.ManifestDigest = digest.MakeSHA256(s.hash)
		s.trackers = append(s.trackers, tracker)
		if s.attestor != nil {
			if err := s.attestor.Register(tracker.Manifest, tracker.ManifestDigest); err != nil {
				return err
			}
		}
		if err := manifest.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (s *DefaultImageScanner) WithProvinanceAttestor(pcr *attest.PathCheckerRegistry) {
	s.attestor = pcr
}

func (s *DefaultImageScanner) GetImages() *types.ImageList {
	images := types.NewImageList(s.directory)
	for _, v := range s.trackers {
		for _, vv := range v.SetValueArgs() {
			name, tag, digest := kimage.Split(vv.Value)
			images.Append(types.Image{
				Sources: []types.Source{{
					ImageSourceLocation: types.ImageSourceLocation{
						Manifest:       v.Manifest,
						ManifestDigest: v.ManifestDigest,
						NodePath:       vv.NodePath,
						Line:           vv.Line,
						Column:         vv.Column,
					},
					OriginalRef: vv.Value,
				}},
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
	s.attestor = nil
}
