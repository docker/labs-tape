package updater

import (
	"sigs.k8s.io/kustomize/api/filters/imagetag"
	// "sigs.k8s.io/kustomize/api/filters/filtersutil"

	"github.com/docker/labs-brown-tape/manifest/types"
)

type Updater struct {
}

func (u *Updater) Update() error {
	_ = imagetag.Filter{
		ImageTag: nil,
		FsSlice:  types.ImagePaths(),
	}

}
