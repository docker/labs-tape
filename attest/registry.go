package attest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"slices"

	"github.com/errordeveloper/tape/attest/digest"
	"github.com/errordeveloper/tape/attest/manifest"
	"github.com/errordeveloper/tape/attest/types"
	"github.com/fxamacker/cbor/v2"
)

type PathCheckerRegistry struct {
	newPathChecker func(string, digest.SHA256) types.PathChecker

	registry     map[types.PathCheckerRegistryKey]types.PathChecker
	mutatedPaths types.Mutations
	statements   types.Statements

	baseDir
}

type baseDir struct {
	pathChecker   types.PathChecker
	cachedSummary types.PathCheckSummary

	fromWorkDir, fromRepoRoot string
}

func NewPathCheckerRegistry(dir string, newPathChecker func(string, digest.SHA256) types.PathChecker) *PathCheckerRegistry {
	return &PathCheckerRegistry{
		baseDir:        baseDir{fromWorkDir: dir},
		newPathChecker: newPathChecker,
		registry:       map[types.PathCheckerRegistryKey]types.PathChecker{},
		statements:     types.Statements{},
	}
}

func (r *PathCheckerRegistry) BaseDirSummary() types.PathCheckSummary {
	return r.baseDir.cachedSummary
}

func (r *PathCheckerRegistry) init(baseDirChecker types.PathChecker) error {
	summary, err := baseDirChecker.MakeSummary()
	if err != nil {
		return fmt.Errorf("unable to make summary for %#v: %w", r.dir(), err)
	}
	r.baseDir = baseDir{
		pathChecker:   baseDirChecker,
		cachedSummary: summary,
		fromRepoRoot:  summary.Common().Path,
		fromWorkDir:   r.fromWorkDir,
	}
	return nil
}

func (r *PathCheckerRegistry) Register(path string, digest digest.SHA256) error {
	key := r.makeKey(r.pathFromRepoRoot(path), digest)
	if _, ok := r.registry[key]; ok {
		return fmt.Errorf("path checker already reigstered for %#v", key)
	}
	r.registry[key] = r.newPathChecker(r.pathFromWorkDir(path), digest)
	return nil
}

func (r *PathCheckerRegistry) RegisterMutated(mutatedPaths types.Mutations) {
	r.mutatedPaths = make(types.Mutations, len(mutatedPaths)) // avoid stale entries
	for k := range mutatedPaths {
		oldDigest := mutatedPaths[k]
		k.Path = r.pathFromRepoRoot(k.Path)
		r.mutatedPaths[k] = oldDigest
	}
}

func (r *PathCheckerRegistry) AssociateStatements(statements ...types.Statement) error {
	for i := range statements {
		if err := statements[i].SetSubjects(func(subject *types.Subject) error {
			path := r.pathFromRepoRoot(subject.Name)
			key := r.makeKey(path, subject.Digest)
			if _, ok := r.registry[key]; !ok {
				err := fmt.Errorf("statement with subject %#v is not relevant (path resoved to %q)", subject, path)
				if r.mutatedPaths == nil {
					return err
				}
				if _, ok := r.mutatedPaths[key]; !ok {
					return err
				}
			}
			subject.Name = path
			return nil
		}); err != nil {
			return err
		}
	}
	r.statements = append(r.statements, statements...)
	return nil
}

func (r *PathCheckerRegistry) MakePathCheckSummarySummaryCollection() (*types.PathCheckSummaryCollection, error) {
	numEntries := len(r.registry) + 1
	if numEntries == 0 {
		return nil, nil
	}
	entries := make([]types.PathChecker, 1, numEntries)
	entries[0] = r.baseDir.pathChecker
	for key := range r.registry {
		entries = append(entries, r.registry[key])
	}
	return types.MakePathCheckSummaryCollection(entries...)
}

func (r *PathCheckerRegistry) AssociateCoreStatements() error {
	errFmt := "unable to associate core statements: %w"

	entries, err := r.MakePathCheckSummarySummaryCollection()
	if err != nil {
		return fmt.Errorf(errFmt, err)
	}

	// this flow is different from AssociateCoreStatements, as path to
	// files is always relative to repo root and statement.SetSubjects
	// doesn't need to be called
	statement := manifest.MakeDirContentsStatement(r.dir(), entries)
	for _, subject := range statement.GetSubject() {
		key := r.makeKey(subject.Name, subject.Digest)
		if _, ok := r.registry[key]; !ok {
			return fmt.Errorf("statement with subject %#v is not relevant", subject)
		}
	}
	r.statements = append(r.statements, statement)

	return nil
}

func (r *PathCheckerRegistry) EncodeAllAttestations(w io.Writer) error {
	encoder := json.NewEncoder(w)
	if err := r.GetStatements().EncodeWith(encoder.Encode); err != nil {
		return fmt.Errorf("unable to encode attestations: %w", err)
	}
	return nil
}

func (r *PathCheckerRegistry) GetStatements() types.Statements {
	slices.SortFunc(r.statements, func(a, b types.Statement) int {
		if cmp := a.Compare(b); cmp != nil {
			return *cmp
		}

		// NB: nil can only be returned if perdicates couldn't be compared,
		// the statement headers were checked first, are always comparable;
		// comparison of bytes obtained from encoding is not ideal, but
		// it's the best we can do as fallback without implementing
		// comparison for each predicate type
		// NB: it's also definite that both of these predicates are of the
		// same type (at least based on the header)
		bufA, bufB := bytes.NewBuffer(nil), bytes.NewBuffer(nil)
		if err := cbor.NewEncoder(bufA).Encode(a.GetPredicate()); err != nil {
			panic(fmt.Sprintf("unexpected error encoding predicate of type %T: %s", a.GetPredicate(), err))
		}
		if err := cbor.NewEncoder(bufB).Encode(b.GetPredicate()); err != nil {
			panic(fmt.Sprintf("unexpected error encoding predicate of type %T: %s", b.GetPredicate(), err))
		}
		return bytes.Compare(bufA.Bytes(), bufB.Bytes())
	})
	return r.statements
}

func (r *PathCheckerRegistry) dir() string {
	switch {
	case r.baseDir.fromRepoRoot != "":
		return r.baseDir.fromRepoRoot
	case r.baseDir.fromWorkDir != "":
		return r.baseDir.fromWorkDir
	default:
		return "."
	}
}

func (r *PathCheckerRegistry) pathFromRepoRoot(path string) string {
	return filepath.Join(r.fromRepoRoot, path)
}

func (r *PathCheckerRegistry) pathFromWorkDir(path string) string {
	return filepath.Join(r.fromWorkDir, path)
}

func (r *PathCheckerRegistry) makeKey(path string, digest digest.SHA256) types.PathCheckerRegistryKey {
	return types.PathCheckerRegistryKey{Path: path, Digest: digest}
}
