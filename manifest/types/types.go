package types

import (
	"path/filepath"

	kustomize "sigs.k8s.io/kustomize/api/types"
)

type (
	Image struct {
		Manifest       string
		ManifestDigest string

		NodePath []string

		OriginalRef  string
		OriginalName string
		OriginalTag  string

		Digest string

		NewName string
		NewTag  string
	}

	ImageList struct {
		items []Image
		dir   string
	}
)

func NewImageList(dir string) *ImageList {
	return &ImageList{
		dir: dir,
	}
}

func (l *ImageList) Paths() []string {
	paths := make([]string, len(l.items))
	for i := range l.items {
		paths[i] = filepath.Join(l.dir, l.items[i].Manifest)
	}
	return paths
}

func (l *ImageList) Items() []Image {
	return l.items
}

func (l *ImageList) Len() int {
	return len(l.items)
}

func (l *ImageList) Append(i Image) {
	l.items = append(l.items, i)
}

func (l *ImageList) GroupByManifest() map[string]*ImageList {
	groups := map[string]*ImageList{}
	for i := range l.items {
		p := filepath.Join(l.dir, l.items[i].Manifest)
		if _, ok := groups[p]; !ok {
			groups[p] = NewImageList(l.dir)
		}
		groups[p].Append(l.items[i])
	}
	return groups
}

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
