package updater

import (
	"crypto/sha256"
	"fmt"
	"hash"

	"sigs.k8s.io/kustomize/api/filters/imagetag"
	kustomize "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"

	"github.com/errordeveloper/tape/attest/digest"
	attestTypes "github.com/errordeveloper/tape/attest/types"
	"github.com/errordeveloper/tape/manifest/types"
	manifestTypes "github.com/errordeveloper/tape/manifest/types"
)

type Updater interface {
	Update(*manifestTypes.ImageList) error
	Mutations() attestTypes.Mutations
}

func NewFileUpdater() Updater {
	return &FileUpdater{
		hash:      sha256.New(),
		mutations: attestTypes.Mutations{},
	}
}

type FileUpdater struct {
	hash      hash.Hash
	mutations attestTypes.Mutations
}

func (u *FileUpdater) Update(images *manifestTypes.ImageList) error {
	groups := images.GroupByManifest()
	for manifestPath := range groups {
		if err := u.doUpdate(manifestPath, groups[manifestPath].Items()); err != nil {
			return err
		}
	}
	return nil
}

func (u *FileUpdater) doUpdate(manifestPath string, images []types.Image) error {
	if len(images) == 0 {
		return fmt.Errorf("no images to update")
	}

	u.hash.Reset()

	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{
			kio.LocalPackageReader{
				PackagePath: manifestPath,
			},
		},
		Filters: make([]kio.Filter, len(images)),
		Outputs: []kio.Writer{
			kio.LocalPackageWriter{
				PackagePath: manifestPath,
			},
			kio.ByteWriter{
				Writer:                u.hash,
				KeepReaderAnnotations: false,
				ClearAnnotations: []string{
					kioutil.PathAnnotation,
					kioutil.LegacyPathAnnotation,
				},
			},
		},
	}

	for i := range images {
		pipeline.Filters[i] = imagetag.Filter{
			ImageTag: kustomize.Image{
				Name:    images[i].OriginalName,
				NewName: images[i].NewName,
				// NB: docs say NewTag is ignored when digest is set, but it's not true
				NewTag: images[i].NewTag,
				Digest: images[i].Digest,
			},
			// this is not optimal, however `(*yaml.RNode).FieldPath()` only returns a flat slice
			// where `contianers[]` is presented as `containers` for some reason; but having
			// a full list of search paths here shouldn't affect performance too much as it's only
			// a short list
			FsSlice: types.ImagePaths(),
		}
	}

	if err := pipeline.Execute(); err != nil {
		return err
	}

	key := attestTypes.PathCheckerRegistryKey{
		Path:   images[0].Manifest(),
		Digest: digest.MakeSHA256(u.hash),
	}
	if _, ok := u.mutations[key]; ok {
		return fmt.Errorf("mutations with key %#v is already registered", key)
	}
	u.mutations[key] = images[0].ManifestDigest()

	return nil
}

func (u *FileUpdater) Mutations() attestTypes.Mutations { return u.mutations }
