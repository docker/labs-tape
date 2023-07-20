package types

import (
	"fmt"
	"path/filepath"
	"sort"
	"sync"

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
		*Source
		Sources []Source

		OriginalName string
		OriginalTag  string

		Digest string

		NewName string
		NewTag  string
	}

	// ImageSource contains fields that are collected from a manifest and will not mutate
	Source struct {
		ImageSourceLocation
		OriginalRef string
	}
	// ImageSourceLocation is a unique location identifier for an image
	ImageSourceLocation struct {
		Manifest       string
		ManifestDigest string

		Line, Column int

		NodePath []string
	}
	ImageList struct {
		items           []Image
		dir             string
		dedupeLock      *sync.Once
		relationEntries map[string]string
	}
)

func NewImageList(dir string) *ImageList {
	return &ImageList{
		items:           []Image{},
		dir:             dir,
		dedupeLock:      &sync.Once{},
		relationEntries: map[string]string{},
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

func (l *ImageList) AppendWithRelationTo(target, image Image) error {
	k := image.Ref(true)
	t := target.Ref(true)
	// fmt.Printf("adding relation for %s => %s\n", k, t)

	if v, ok := l.relationEntries[k]; ok {
		return fmt.Errorf("unexpected duplicate relation entry for %q (current value %q, new value %q)", k, v, t)
	}
	l.relationEntries[k] = t
	l.Append(image)
	return nil
}

// func (l *ImageList) IsRelated(key, target string) bool {
// 	if l.relationEntries == nil {
// 		return false
// 	}
// 	v, ok := l.relationEntries[key]
// 	if !ok {
// 		return false
// 	}
// 	return v == target
// }

func (l *ImageList) RelatedTo(ref string) []string {
	results := []string{}
	for k, v := range l.relationEntries {
		if v == ref {
			results = append(results, k)
		}
	}
	// fmt.Printf("returning relations for %v \n", results)
	return results
}

func (l *ImageList) GetItemByRef(ref string) *Image {
	for i := range l.items {
		if l.items[i].Ref(true) == ref {
			return &l.items[i]
		}
	}
	return nil
}

func (l *ImageList) GetItemByDigest(digest string) *Image {
	for i := range l.items {
		if l.items[i].Digest == digest {
			return &l.items[i]
		}
	}
	return nil
}

func (l *ImageList) CollectRelatedToRef(ref string) *ImageList {
	result := NewImageList(l.Dir())
	for _, relatedRef := range l.RelatedTo(ref) {
		if relatedImage := l.GetItemByRef(relatedRef); relatedImage != nil {
			result.Append(*relatedImage)
		}
	}
	return result
}

func (l *ImageList) Dedup() error {
	type key [2]string

	for _, image := range l.items {
		if image.Digest == "" {
			return fmt.Errorf("image %s has no digest", image.Ref(true))
		}
	}
	l.dedupeLock.Do(func() {
		unique := map[key]Image{}
		for _, image := range l.items {
			sources := []Source{*image.Source}
			existing, present := unique[key{image.OriginalRef, image.Digest}]
			if present {
				sources = append(sources, existing.Sources...)
			}
			unique[key{image.OriginalRef, image.Digest}] = Image{
				Sources:      sources,
				OriginalTag:  image.OriginalTag,
				OriginalName: image.OriginalName,
				Digest:       image.Digest,
				NewName:      image.NewName,
				NewTag:       image.NewTag,
			}
		}

		l.items = make([]Image, 0, len(unique))
		for _, v := range unique {
			if len(v.Sources) > 1 {
				sort.Slice(v.Sources, func(i, j int) bool {
					if v.Sources[i].Manifest == v.Sources[j].Manifest {
						if v.Sources[i].Line == v.Sources[j].Line {
							return v.Sources[i].Column < v.Sources[j].Column
						}
						return v.Sources[i].Line < v.Sources[j].Line
					}
					return v.Sources[i].Manifest < v.Sources[j].Manifest
				})
			}
			l.items = append(l.items, v)
		}
	})
	return nil
}

func (l *ImageList) Len() int {
	return len(l.items)
}

func (l *ImageList) Append(i ...Image) {
	if len(i) == 0 {
		return
	}
	l.items = append(l.items, i...)
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
