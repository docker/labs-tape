package image

import (
	"cmp"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"sync"
)

type ImageList struct {
	items           []Image
	dir             string
	dedupeLock      *sync.Once
	relationEntries map[string]string
}

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
		paths[i] = filepath.Join(l.dir, l.items[i].Manifest())
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
			sources := slices.Clone(image.Sources)
			k := key{image.OriginalRef(), image.Digest}
			existing, present := unique[k]
			if present {
				sources = append(sources, existing.Sources...)
			}
			unique[k] = Image{
				Sources:      sources,
				OriginalTag:  image.OriginalTag,
				OriginalName: image.OriginalName,
				Digest:       image.Digest,
				NewName:      image.NewName,
				NewTag:       image.NewTag,
			}
		}

		l.items = make([]Image, 0, len(unique))
		for k := range unique {
			v := unique[k]
			if len(v.Sources) > 1 {
				slices.SortFunc(v.Sources, func(a, b Source) int {
					if cmp := cmp.Compare(a.Manifest, b.Manifest); cmp != 0 {
						return cmp
					}
					if cmp := cmp.Compare(a.Line, b.Line); cmp != 0 {
						return cmp
					}
					if cmp := cmp.Compare(a.Column, b.Column); cmp != 0 {
						return cmp
					}
					return 0
				})
			}
			l.items = append(l.items, v)
		}
	})

	l.MakeAliases()

	return nil
}

func (l *ImageList) MakeAliases() {
	aliases := NewAliasCache(l.items).MakeAliasesForNames()
	for i := range aliases {
		l.items[i].Alias = &aliases[i]
	}
}

func (l *ImageList) Len() int { return len(l.items) }

func (l *ImageList) Append(i ...Image) {
	if len(i) == 0 {
		return
	}
	l.items = append(l.items, i...)
}

func (l *ImageList) GroupByManifest() map[string]*ImageList {
	groups := map[string]*ImageList{}
	addItem := func(manifest string, item Image) {
		p := filepath.Join(l.dir, manifest)
		if _, ok := groups[p]; !ok {
			groups[p] = NewImageList(l.dir)
		}
		groups[p].Append(item)
	}
	for i := range l.items {
		item := l.items[i]
		for _, source := range item.Sources {
			addItem(source.Manifest, Image{
				Sources:      []Source{source},
				OriginalName: item.OriginalName,
				OriginalTag:  item.OriginalTag,
				Digest:       item.Digest,
				NewName:      item.NewName,
				NewTag:       item.NewTag,
			})
		}
	}
	return groups
}

func (l *ImageList) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.items)
}
