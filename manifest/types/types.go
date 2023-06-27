package types

import (
	kustomize "sigs.k8s.io/kustomize/api/types"
)

type Image struct {
	Manifest       string
	ManifestDigest string

	NodePath []string

	OriginalRef  string
	OriginalName string
	OriginalTag  string

	Digest string
	NewRef string
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
