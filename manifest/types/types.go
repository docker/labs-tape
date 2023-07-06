package types

import (
	"path/filepath"

	kustomize "sigs.k8s.io/kustomize/api/types"
)

const (
	AppImageTagPrefix    = "app."
	ConfigImageTagPrefix = "config."
)

type (
	// TODO: this is not optimal as resolution and copying ends up being done for each item,
	// and those can be repeated a few times;  the structure should evolve so that there is
	// a unique entry with references to multiple origins
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

func (l *ImageList) Dir() string {
	return l.dir
}

func (l *ImageList) Items() []Image {
	return l.items
}

func (l *ImageList) UniqueItems() []Image {
	type key [2]string
	unique := map[key]Image{}
	for _, image := range l.items {
		unique[key{image.OriginalRef, image.Digest}] = Image{
			OriginalRef:  image.OriginalRef,
			OriginalTag:  image.OriginalTag,
			OriginalName: image.OriginalName,
			Digest:       image.Digest,
			NewName:      image.NewName,
			NewTag:       image.NewTag,
		}
	}

	items := []Image{}
	for _, v := range unique {
		items = append(items, v)
	}
	return items
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

func (i Image) Ref(original bool) string {
	ref := ""
	if original {
		ref = i.OriginalName
		if i.OriginalTag != "" {
			ref += ":" + i.OriginalTag
		}
	} else {
		ref = i.NewName
		if i.NewTag != "" {
			ref += ":" + i.NewTag
		}
	}
	if i.Digest != "" {
		ref += "@" + i.Digest
	}
	return ref
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
