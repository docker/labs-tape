package git

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/utils/ioutil"

	"github.com/docker/labs-brown-tape/attest/digest"
	"github.com/docker/labs-brown-tape/attest/types"
)

const (
	ProviderName = "git"

	DefaultPrimaryRemoteName = "origin"
)

func NewPathChecker(path string, digest digest.SHA256) types.PathChecker {
	return &PathChecker{
		path:   path,
		digest: digest,
	}
}

type PathChecker struct {
	path   string
	digest digest.SHA256
	cache  *pathCheckerCache
}

type (
	Summary struct {
		types.PathCheckSummaryCommon `json:",inline"`

		Git *GitSummary `json:"git,omitempty"`
	}

	GitSummary struct {
		Object    GitObject           `json:"object,omitempty"`
		Remotes   map[string][]string `json:"remotes,omitempty"`
		Reference GitReference        `json:"reference,omitempty"`
	}

	GitObject struct {
		TreeHash   string `json:"treeHash,omitempty"`
		CommitHash string `json:"commitHash,omitempty"`
	}

	Signature struct {
		PGP       []byte `json:"pgp"`
		Validated bool   `json:"validated"`
	}

	GitTag struct {
		Name      string     `json:"name"`
		Hash      string     `json:"hash,omitempty"`
		Target    string     `json:"target,omitempty"`
		Signature *Signature `json:"signature,omitempty"`
	}

	GitReference struct {
		Name      string     `json:"name,omitempty"`
		Hash      string     `json:"hash,omitempty"`
		Type      string     `json:"type,omitempty"`
		Target    string     `json:"target,omitempty"`
		Tags      []GitTag   `json:"tags,omitempty"`
		Signature *Signature `json:"signature,omitempty"`
	}
)

type pathCheckerCache struct {
	checked    bool
	unmodified bool
	absPath    string
	repo       *gogit.Repository
	obj        object.Object
	blobHash   string
	repoPath   string
}

func (s *Summary) SameRepo(other types.PathCheckSummary) bool {
	if other.ProviderName() != ProviderName {
		return false
	}

	if other, ok := other.Full().(*Summary); !ok || (ok && evalChecks(true,
		(s.URI != other.URI),
		func() bool { return (s.Git == nil || other.Git == nil) },
		func() bool { return (s.Git.Reference.Hash != other.Git.Reference.Hash) },
		func() bool { return (len(s.Git.Remotes) != len(other.Git.Remotes)) },
	)) {
		return false
	}

	return true
}

func evalChecks(v bool, args ...any) bool {
	for _, c := range makeChecks(args...) {
		if c.eval() == v {
			return true
		}
	}
	return false
}

func makeChecks(args ...any) checks {
	checks := make(checks, len(args))
	for i := range args {
		switch arg := any(args[i]).(type) {
		case bool:
			checks[i] = checkVal(arg)
		case func() bool:
			checks[i] = checkFunc(arg)
		}
	}
	return checks
}

type checks []interface{ eval() bool }

type checkVal bool

func (v checkVal) eval() bool { return bool(v) }

type checkFunc func() bool

func (f checkFunc) eval() bool { return f() }

func (c *PathChecker) MakeSummary() (types.PathCheckSummary, error) {
	if c.cache == nil || !c.cache.checked {
		checked, _, err := c.Check()
		if err != nil {
			return nil, err
		}
		if !checked {
			return nil, fmt.Errorf("%q is not checked", c.path)
		}
	}

	git := GitSummary{}

	summary := &Summary{
		PathCheckSummaryCommon: types.PathCheckSummaryCommon{
			Unmodified: c.cache.unmodified,
			Path:       c.cache.repoPath,
			IsDir:      c.IsTree(),
			Digest:     c.digest,
		},
		Git: &git,
	}

	head, err := c.cache.repo.Head()
	if err != nil {
		return nil, err
	}

	ref := GitReference{
		Name:   head.Name().String(),
		Hash:   head.Hash().String(),
		Type:   head.Type().String(),
		Target: head.Target().String(),
	}

	obj := &GitObject{}
	if summary.Unmodified {
		obj.TreeHash = c.cache.obj.ID().String()
	} else if c.IsBlob() {
		// there is currently no easy way to obtain a hash for a subtree
		obj.TreeHash = c.cache.blobHash
	}

	headCommit, err := c.cache.repo.CommitObject(head.Hash())
	if err != nil {
		return nil, err
	}
	if headCommit.PGPSignature != "" {
		ref.Signature = &Signature{
			PGP:       []byte(headCommit.PGPSignature),
			Validated: false,
		}
	}

	if summary.Unmodified {
		commitIter := object.NewCommitPathIterFromIter(
			func(path string) bool {
				switch {
				case c.IsTree():
					return strings.HasPrefix(c.cache.repoPath, path)
				case c.IsBlob():
					return c.cache.repoPath == path
				default:
					return false
				}
			},
			object.NewCommitIterCTime(headCommit, nil, nil),
			true,
		)
		defer commitIter.Close()
		// only need first commit, avoid looping over all commits with ForEach
		commit, err := commitIter.Next()
		if err == nil {
			obj.CommitHash = commit.Hash.String()
		} else if err != io.EOF {
			return nil, err
		}
	}

	tags, err := c.cache.repo.Tags()
	if err != nil {
		return nil, err
	}

	if err := tags.ForEach(func(t *plumbing.Reference) error {
		target, err := c.cache.repo.ResolveRevision(plumbing.Revision(t.Name()))
		if err != nil {
			return err
		}
		if *target != head.Hash() {
			// doesn't point to HEAD
			return nil
		}

		tag := GitTag{
			Name:   t.Name().Short(),
			Hash:   t.Hash().String(),
			Target: target.String(),
		}

		if tag.Target != tag.Hash {
			// annotated tags have own object hash, while has of a leightweight tag is the same as target
			tagObject, err := c.cache.repo.TagObject(t.Hash())
			if err != nil {
				return err
			}
			if tagObject.PGPSignature != "" {
				tag.Signature = &Signature{
					PGP:       []byte(tagObject.PGPSignature),
					Validated: false,
				}
			}
		}

		ref.Tags = append(ref.Tags, tag)
		return nil
	}); err != nil {
		return nil, err
	}

	remotes, err := c.cache.repo.Remotes()
	if err != nil {
		return nil, err
	}
	numRemotes := len(remotes)
	primatyURLs := []string{}
	if numRemotes > 0 {
		primaryRemoteIndex := 0
		// attempt to find remote by default name
		for i, remote := range remotes {
			if remote.Config().Name == DefaultPrimaryRemoteName {
				primaryRemoteIndex = i
			}
		}
		// fallback to first entry
		primatyURLs = append(primatyURLs, remotes[primaryRemoteIndex].Config().URLs...)
	}

	if len(primatyURLs) > 0 {
		ep, err := transport.NewEndpoint(primatyURLs[0])
		if err != nil {
			return nil, err
		}

		summary.URI = ep.String()
	}
	git.Remotes = make(map[string][]string, numRemotes)
	for _, remote := range remotes {
		remoteConfig := remote.Config()
		git.Remotes[remoteConfig.Name] = remoteConfig.URLs
	}

	git.Reference = ref
	git.Object = *obj

	return summary, nil
}

func (PathChecker) ProviderName() string { return ProviderName }

func (s *Summary) Full() interface{} { return s }

func (c *Summary) ProviderName() string { return ProviderName }

func (c *PathChecker) DetectRepo() (bool, error) {
	if c.cache == nil {
		c.cache = &pathCheckerCache{}
	}

	absPath, err := filepath.Abs(c.path)
	if err != nil {
		return false, err
	}
	repo, ok := detectRepo(absPath)
	if !ok {
		return false, nil
	}

	c.cache.absPath = absPath
	c.cache.repo = repo

	return ok, nil
}

func (c *PathChecker) Check() (bool, bool, error) {
	if c.cache == nil {
		c.cache = &pathCheckerCache{}
	}
	if c.cache.repo == nil {
		ok, err := c.DetectRepo()
		if err != nil {
			return c.negative(err)
		}
		if !ok {
			return c.negative(nil)
		}
	}

	worktree, err := c.cache.repo.Worktree()
	if err != nil {
		return c.negative(err)
	}
	repoPath, err := filepath.Rel(worktree.Filesystem.Root(), c.cache.absPath)
	if err != nil {
		return c.negative(err)
	}

	c.cache.repoPath = repoPath

	obj, err := findByPath(c.cache.repo, repoPath)
	if err != nil {
		return c.negative(err)
	}
	if obj == nil {
		return c.negative(nil)
	}

	c.cache.obj = obj
	c.cache.checked = true

	switch obj.Type() {
	case plumbing.BlobObject:
		unmodified, blobHash, err := isBlobUnmodified(worktree, obj.(*object.Blob), repoPath)
		if err != nil {
			return c.negative(err)
		}

		c.cache.unmodified = unmodified
		c.cache.blobHash = blobHash

		return c.cache.checked, c.cache.unmodified, nil
	case plumbing.TreeObject:
		tree := obj.(*object.Tree)
		c.cache.unmodified = true

		if err := tree.Files().ForEach(func(f *object.File) error {
			// no need to check each file if one was already modified
			if !c.cache.unmodified {
				return nil
			}
			unmodified, _, err := isBlobUnmodified(worktree, &f.Blob, filepath.Join(repoPath, f.Name))
			if err != nil {
				return err
			}
			if !unmodified {
				c.cache.unmodified = false
			}
			return nil
		}); err != nil {
			return c.negative(err)
		}

		// if modification is already detect, return early
		if !c.cache.unmodified {
			return c.cache.checked, c.cache.unmodified, nil
		}

		// this is not very fast, as it needs to inspect entire worktree,
		// so only resort to it when all files in subdir are checked
		status, err := worktree.Status()
		if err != nil {
			return c.negative(err)
		}
		for filePath, fileStatus := range status {
			if fileStatus.Staging == gogit.Unmodified && fileStatus.Worktree == gogit.Unmodified {
				// skip any unmodified files
				continue
			}

			// ingnore file in other directories
			if !fileIsInDir(filePath, repoPath) {
				continue
			}

			c.cache.unmodified = false
			break // only need to detect first modified file
		}

		return c.cache.checked, c.cache.unmodified, nil

	default:
		return c.negative(fmt.Errorf("unsupported object type: %q", obj.Type().String()))
	}
}

func (c *PathChecker) Reset() { c.cache = nil }

func fileIsInDir(file, dir string) bool {
	rel, err := filepath.Rel(dir, file)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	relParts := strings.Split(rel, string(filepath.Separator))
	if len(relParts) == 0 {
		return false
	}
	return relParts[0] != ".."
}

func isBlobUnmodified(worktree *gogit.Worktree, blob *object.Blob, repoPath string) (_ bool, _ string, err error) {
	// there is blob.Reader(), however it reads checked contents, while for this check
	// a hash of working tree contents is needed
	file, err := worktree.Filesystem.Open(repoPath)
	if err != nil {
		return false, "", err
	}
	defer ioutil.CheckClose(file, &err)

	data, err := io.ReadAll(file)
	if err != nil {
		return false, "", err
	}
	blobHash := plumbing.ComputeHash(plumbing.BlobObject, data).String()
	return blobHash == blob.Hash.String(), blobHash, nil
}

func (c *PathChecker) Repository() *gogit.Repository {
	if c.cache == nil {
		return nil
	}
	return c.cache.repo
}

func (c *PathChecker) IsTree() bool {
	if c.cache == nil || c.cache.obj == nil {
		return false
	}
	return c.cache.obj.Type() == plumbing.TreeObject
}

func (c *PathChecker) IsBlob() bool {
	if c.cache == nil || c.cache.obj == nil {
		return false
	}
	return c.cache.obj.Type() == plumbing.BlobObject
}

func (c *PathChecker) Blob() *object.Blob {
	if c.cache == nil ||
		c.cache.obj == nil ||
		c.cache.obj.Type() != plumbing.BlobObject {
		return nil
	}
	return c.cache.obj.(*object.Blob)
}

func (c *PathChecker) Tree() *object.Tree {
	if c.cache == nil ||
		c.cache.obj == nil ||
		c.cache.obj.Type() != plumbing.TreeObject {
		return nil
	}
	return c.cache.obj.(*object.Tree)
}

func (c *PathChecker) negative(err error) (bool, bool, error) {
	c.Reset()
	return false, false, err
}

func findByPath(repo *gogit.Repository, path string) (object.Object, error) {
	head, err := repo.Head()
	if err != nil {
		return nil, err
	}
	commitObj, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, err
	}
	tree, err := commitObj.Tree()
	if err != nil {
		return nil, err
	}
	treeEntry, err := tree.FindEntry(path)
	switch err {
	case nil:
		switch treeEntry.Mode {
		case filemode.Regular, filemode.Deprecated, filemode.Executable:
			return repo.BlobObject(treeEntry.Hash)
		case filemode.Dir:
			return repo.TreeObject(treeEntry.Hash)
		default:
			return nil, fmt.Errorf("unsupported mode: %q %q", path, treeEntry.Mode.String())
		}
	case object.ErrDirectoryNotFound, object.ErrFileNotFound, plumbing.ErrObjectNotFound, object.ErrEntryNotFound:
		return nil, nil
	default:
		return nil, err
	}
}

func detectRepo(path string) (*gogit.Repository, bool) {
	dir := filepath.Dir(path)
	if dir == path { // reached root
		return nil, false
	}
	if repo, err := gogit.PlainOpen(dir); err == nil {
		return repo, true
	}
	return detectRepo(dir)
}
