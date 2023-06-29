package updater

import (
	"sigs.k8s.io/kustomize/api/filters/imagetag"
	kustomize "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/kio"

	"github.com/docker/labs-brown-tape/manifest/types"
)

// TODO(attest): provide an attestation for each new manifest and reference checksum of the original manifest

type Updater interface {
	Update(*types.ImageList) error
}

func NewFileUpdater() Updater { return &FileUpdater{} }

type FileUpdater struct{}

func (u *FileUpdater) Update(images *types.ImageList) error {
	for manifest, images := range images.GroupByManifest() {
		if err := u.doUpdate(manifest, images.Items()); err != nil {
			return err
		}
	}
	return nil
}

func (u *FileUpdater) doUpdate(manifest string, images []types.Image) error {
	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{
			kio.LocalPackageReader{
				PackagePath: manifest,
			},
		},
		Filters: make([]kio.Filter, len(images)),
		Outputs: []kio.Writer{
			kio.LocalPackageWriter{
				PackagePath: manifest,
			},
		},
	}

	for i, image := range images {
		pipeline.Filters[i] = imagetag.Filter{
			ImageTag: kustomize.Image{
				Name:    image.OriginalName,
				NewName: image.NewName,
				// NB: docs say NewTag is ignored when digest is set, but it's not true
				NewTag: image.NewTag,
				Digest: image.Digest,
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
	return nil
}
