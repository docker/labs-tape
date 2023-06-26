package updater

import (
	"os"

	"sigs.k8s.io/kustomize/api/filters/imagetag"
	"sigs.k8s.io/kustomize/kyaml/kio"

	// "sigs.k8s.io/kustomize/api/filters/filtersutil"
	kustomize "sigs.k8s.io/kustomize/api/types"

	"github.com/docker/labs-brown-tape/manifest/types"
)

// TODO(attest): provide an attestation for each new manifest and reference checksum of the original manifest

type Updater struct {
}

func (u *Updater) Update(images []types.Image) error {
	for i := range images {
		if err := u.doUpdate(&images[i]); err != nil {
			return err
		}
	}
	return nil
}

func (u *Updater) doUpdate(i *types.Image) error {
	filter := imagetag.Filter{
		ImageTag: kustomize.Image{
			Name:    i.OriginalName,
			NewName: i.NewName,
			NewTag:  i.NewTag,
			Digest:  i.Digest,
		},
		FsSlice: types.ImagePaths(),
	}

	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{
			kio.LocalPackageReader{
				PackagePath: i.Manifest,
			},
		},
		Filters: []kio.Filter{filter},
		Outputs: []kio.Writer{
			&kio.ByteWriter{Writer: os.Stdout},
			// kio.LocalPackageWriter{
			// 	PackagePath: i.Manifest,
			// },
		},
	}

	if err := pipeline.Execute(); err != nil {
		return err
	}
	return nil
}
