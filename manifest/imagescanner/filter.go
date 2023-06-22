package imagescanner

import (
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"

	"sigs.k8s.io/kustomize/api/filters/filtersutil"
	"sigs.k8s.io/kustomize/api/filters/fsslice"
)

type Filter struct {
	trackableSetter filtersutil.TrackableSetter
}

var _ kio.Filter = Filter{}
var _ kio.TrackableFilter = &Filter{}

func (f *Filter) WithMutationTracker(callback func(key, value, tag string, node *yaml.RNode)) {
	f.trackableSetter.WithMutationTracker(callback)
}

func (f Filter) Filter(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
	_, err := kio.FilterAll(yaml.FilterFunc(f.filter)).Filter(nodes)
	return nodes, err
}

func (f Filter) filter(node *yaml.RNode) (*yaml.RNode, error) {
	// FsSlice is an allowlist, not a denyList, so to deny
	// something via configuration a new config mechanism is
	// needed. Until then, hardcode it.
	if f.isOnDenyList(node) {
		return node, nil
	}
	if err := node.PipeE(fsslice.Filter{
		FsSlice: []types.FieldSpec{
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
		},
		SetValue: f.SetValue,
	}); err != nil {
		return nil, err
	}
	return node, nil
}

func (f Filter) isOnDenyList(node *yaml.RNode) bool {
	meta, err := node.GetMeta()
	if err != nil {
		// A missing 'meta' field will cause problems elsewhere;
		// ignore it here to keep the signature simple.
		return false
	}
	// Ignore CRDs
	// https://github.com/kubernetes-sigs/kustomize/issues/890
	return meta.Kind == `CustomResourceDefinition`
}

func (f Filter) SetValue(rn *yaml.RNode) error {
	if err := yaml.ErrorIfInvalid(rn, yaml.ScalarNode); err != nil {
		return err
	}

	value := rn.YNode().Value

	f.trackableSetter.SetScalar(value)(rn)
	return nil
}

type SetValueArg struct {
	Key      string
	Value    string
	Tag      string
	NodePath []string
}

type Tracker struct {
	Manifest       string
	ManifestDigest string

	setValueArgs []SetValueArg
}

func (t *Tracker) MutationTracker(key, value, tag string, node *yaml.RNode) {
	t.setValueArgs = append(t.setValueArgs, SetValueArg{
		Key:      key,
		Value:    value,
		Tag:      tag,
		NodePath: node.FieldPath(),
	})
}

func (t *Tracker) SetValueArgs() []SetValueArg {
	return t.setValueArgs
}

func (t *Tracker) Reset() {
	t.setValueArgs = nil
}
