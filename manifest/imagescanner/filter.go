package imagescanner

import (
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"

	"sigs.k8s.io/kustomize/api/filters/filtersutil"
	"sigs.k8s.io/kustomize/api/filters/fsslice"

	"github.com/errordeveloper/tape/attest/digest"
	"github.com/errordeveloper/tape/manifest/types"
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
		FsSlice:  types.ImagePaths(),
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
	Key, Value, Tag string
	Line, Column    int
	NodePath        []string
}

type Tracker struct {
	Manifest       string
	ManifestDigest digest.SHA256

	setValueArgs []SetValueArg
}

func (t *Tracker) MutationTracker(key, value, tag string, node *yaml.RNode) {
	t.setValueArgs = append(t.setValueArgs, SetValueArg{
		Key:      key,
		Value:    value,
		Tag:      tag,
		NodePath: node.FieldPath(),
		Line:     node.Document().Line,
		Column:   node.Document().Column,
	})
}

func (t *Tracker) SetValueArgs() []SetValueArg {
	return t.setValueArgs
}

func (t *Tracker) Reset() {
	t.setValueArgs = nil
}
