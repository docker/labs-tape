package types

import (
	kustomize "sigs.k8s.io/kustomize/api/types"

	"github.com/errordeveloper/tape/manifest/image"
)

const (
	AppImageTagPrefix    = "app."
	ConfigImageTagPrefix = "config."
)

type (
	Image               = image.Image
	Source              = image.Source
	ImageSourceLocation = image.ImageSourceLocation
	ImageList           = image.ImageList
)

func NewImageList(dir string) *ImageList { return image.NewImageList(dir) }

func ImagePaths() []kustomize.FieldSpec {
	return []kustomize.FieldSpec{
		{Path: "spec/containers[]/image"},
		{Path: "spec/initContainers[]/image"},
		{Path: "spec/template/spec/containers[]/image"},
		{Path: "spec/template/spec/initContainers[]/image"},
		// kustomize can process flat lists, but not nested lists,
		// these paths enable 1 level of nesting
		// TODO: find a better way to address it for arbitrary depths
		{Path: "items[]/spec/containers[]/image"},
		{Path: "items[]/spec/initContainers[]/image"},
		{Path: "items[]/spec/template/spec/containers[]/image"},
		{Path: "items[]/spec/template/spec/initContainers[]/image"},
	}
}
