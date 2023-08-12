package attest

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/docker/labs-brown-tape/attest/digest"
	"github.com/docker/labs-brown-tape/attest/manifest"
	"github.com/docker/labs-brown-tape/attest/types"
)

type PathCheckerRegistryKey struct {
	Path   string
	Digest digest.SHA256
}

type PathCheckerRegistry struct {
	newPathChecker func(string, digest.SHA256) types.PathChecker

	registry   map[PathCheckerRegistryKey]types.PathChecker
	statements types.Statements

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
		registry:       map[PathCheckerRegistryKey]types.PathChecker{},
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

func (r *PathCheckerRegistry) AssociateStatements(statements ...types.Statement) error {
	for i := range statements {
		if err := statements[i].SetSubjects(func(subject *types.Subject) error {
			path := r.pathFromRepoRoot(subject.Name)
			key := r.makeKey(path, subject.Digest)
			if _, ok := r.registry[key]; !ok {
				return fmt.Errorf("statement with subject %#v is not relevant (path resoved to %q)", subject, path)
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
	if err := r.statements.EncodeWith(encoder.Encode); err != nil {
		return fmt.Errorf("unable to encode attestations: %w", err)
	}
	return nil
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

func (r *PathCheckerRegistry) makeKey(path string, digest digest.SHA256) PathCheckerRegistryKey {
	return PathCheckerRegistryKey{Path: path, Digest: digest}
}
