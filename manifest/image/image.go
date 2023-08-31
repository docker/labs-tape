package image

import (
	kustomize "sigs.k8s.io/kustomize/api/types"

	"github.com/docker/labs-brown-tape/attest/digest"
)

type (
	Image struct {
		Sources []Source

		OriginalName, OriginalTag string

		Digest string

		NewTag, NewName string

		Alias *string
	}

	// ImageSource contains fields that are collected from a manifest and will not mutate
	Source struct {
		ImageSourceLocation
		OriginalRef string
	}
	// ImageSourceLocation is a unique location identifier for an image
	ImageSourceLocation struct {
		Manifest       string
		ManifestDigest digest.SHA256

		Line, Column int

		NodePath []string
	}
)

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

func (i Image) primarySource() *Source {
	if len(i.Sources) == 0 {
		panic("unextected empty image sources")
	}
	return &i.Sources[0]
}

func (i Image) Manifest() string              { return i.primarySource().Manifest }
func (i Image) ManifestDigest() digest.SHA256 { return i.primarySource().ManifestDigest }

func (i Image) OriginalRef() string { return i.primarySource().OriginalRef }

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
