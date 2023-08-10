package attest

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/docker/labs-brown-tape/attest/digest"
	"github.com/docker/labs-brown-tape/attest/types"
)

type PathCheckerRegistryKey struct {
	Path   string
	Digest digest.SHA256
}

type PathCheckerRegistry struct {
	newPathChecker func(string) types.PathChecker
	baseDir        string
	registry       map[PathCheckerRegistryKey]types.PathChecker
	statements     map[PathCheckerRegistryKey]types.Statements
}

func NewPathCheckerRegistry(dir string, newPathChecker func(string) types.PathChecker) *PathCheckerRegistry {
	return &PathCheckerRegistry{
		newPathChecker: newPathChecker,
		baseDir:        dir,
		registry:       map[PathCheckerRegistryKey]types.PathChecker{},
		statements:     map[PathCheckerRegistryKey]types.Statements{},
	}
}

func (r *PathCheckerRegistry) Init() error {
	dir := r.baseDir
	if dir == "" {
		dir = "."
	}
	if err := r.Register(dir, ""); err != nil {
		return fmt.Errorf("unable to initialise path checker registry: %w", err)
	}
	return nil
}

func (r *PathCheckerRegistry) makeKey(path string, digest digest.SHA256) PathCheckerRegistryKey {
	if r.baseDir != "" && r.baseDir != path {
		path = filepath.Join(r.baseDir, path)
	}
	return PathCheckerRegistryKey{Path: path, Digest: digest}
}

func (r *PathCheckerRegistry) Register(path string, digest digest.SHA256) error {
	key := r.makeKey(path, digest)
	if _, ok := r.registry[key]; ok {
		return fmt.Errorf("path checker already reigstered for %#v", key)
	}
	r.registry[key] = r.newPathChecker(key.Path)
	return nil
}

func (r *PathCheckerRegistry) AssociateStatements(statements ...types.Statement) error {
	for _, statement := range statements {
		key := r.makeKey(statement.GetSubjectName(), statement.GetSubjectDigest())
		if _, ok := r.registry[key]; !ok {
			return fmt.Errorf("path checker not reigstered for %#v", key)
		}
		r.statements[key] = append(r.statements[key], statement)
	}
	return nil
}

func (r *PathCheckerRegistry) EncodeAll(w io.Writer) error {
	encoder := json.NewEncoder(w)
	for key, checker := range r.registry {
		doEncode := func(obj interface{}) error {
			k := key.Path
			if key.Digest != "" {
				k += "@" + key.Digest.String()
			}
			switch obj := obj.(type) {
			case types.Statements:
				if err := obj.EncodeWith(encoder.Encode); err != nil {
					return fmt.Errorf("unable to encode attestation for %q: %w", k, err)
				}
			case types.PathCheckSummary:
				if err := encoder.Encode(map[string]interface{}{k: obj.Full()}); err != nil {
					return fmt.Errorf("unable to encode VCS summary for %q: %w", k, err)
				}
			}
			return nil
		}
		summary, err := checker.MakeSummary()
		if err != nil {
			return fmt.Errorf("unable to make summary for %v: %w", key, err)
		}
		if err := doEncode(summary.Full()); err != nil {
			return err
		}
		statements, ok := r.statements[key]
		if !ok {
			continue
		}
		if err := doEncode(statements); err != nil {
			return err
		}
	}

	return nil
}
