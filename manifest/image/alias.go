package image

import (
	"fmt"
	"slices"
	"strings"

	kimage "sigs.k8s.io/kustomize/api/image"
)

const (
	separator = "/"
)

type imageList interface {
	[]string | []Image
}

func NewAliasCache[T imageList](imageNames T) AliasCache {
	names := make(AliasCache, len(imageNames))
	switch imageNames := any(imageNames).(type) {
	case []string:
		for i := range imageNames {
			names[i] = newImageName(kimage.Split(imageNames[i]))
		}
	case []Image:
		for i := range imageNames {
			names[i] = newImageName(imageNames[i].OriginalName, imageNames[i].OriginalTag, imageNames[i].Digest)
		}
		return names
	default:
		panic(fmt.Sprintf("unhandled type %T", imageNames))
	}
	return names
}

type AliasCache []*imageName

func (l AliasCache) Match(name string) (string, []string, bool) {
	candidates := []string{}

	for i := range l {
		switch name {
		case l[i].join(), l[i].longest():
			candidates = append(candidates, l[i].longest())
		}
	}

	if len(candidates) == 0 {
		parts := strings.Split(name, separator)
		for i := range l {
			for j := range l[i].parts {
				if slices.Compare(l[i].parts[j:], parts) == 0 {
					candidates = append(candidates, l[i].longest())
				}
			}
		}
	}

	switch len(candidates) {
	case 1:
		return candidates[0], nil, true
	case 0:
		return "", nil, false
	default:
		return "", candidates, false
	}
}

func (l AliasCache) MakeAliasesForNames() []string {
	names := make([]string, len(l))
	for i := range l {
		names[i] = l[i].shortest()
	}

	for {
		mutated := false
		for n := range names {
			for m := range names {
				if m == n || equalParts(l[m], l[n]) {
					continue
				}

				if names[m] == names[n] {
					if l[m].extend() {
						names[m] = l[m].join()
						mutated = true
					}
					if l[n].extend() {
						names[n] = l[n].join()
						mutated = true
					}
				}

				if extendIfSuffixMatches(l[m], l[n]) {
					names[m] = l[m].join()
					names[n] = l[n].join()

					mutated = true
					continue
				}

			}
		}
		if !mutated {
			break
		}
	}
	return names
}

func newImageName(name, tag, digest string) *imageName {
	nameParts := strings.Split(name, separator)
	last := len(nameParts) - 1
	return &imageName{
		parts:   nameParts,
		tag:     tag,
		digest:  digest,
		last:    last,
		current: last,
	}
}

type imageName struct {
	parts         []string
	tag, digest   string
	last, current int
}

func (i *imageName) shortest() string { return i.parts[i.last] }
func (i *imageName) longest() string  { return strings.Join(i.parts, separator) }
func (i *imageName) join() string     { return strings.Join(i.parts[i.current:], separator) }

func (i *imageName) extendable() bool { return i.current > 0 }
func (i *imageName) extend() bool {
	if !i.extendable() {
		return false
	}
	i.current--
	return true
}

func equalParts(a, b *imageName) bool { return slices.Compare(a.parts, b.parts) == 0 }

func extendIfSuffixMatches(a, b *imageName) bool {
	if !a.extendable() {
		return false
	}
	mutated := false
	if slices.Compare(a.parts[a.current-1:], b.parts[b.current:]) == 0 {
		if a.extend() {
			mutated = true
		}
		if b.extend() {
			mutated = true
		}
	}
	for i := b.last; i > b.current; i-- {
		if slices.Compare(a.parts[a.current:], b.parts[i:]) == 0 {
			if a.extend() {
				mutated = true
			}
		}
	}
	return mutated
}
