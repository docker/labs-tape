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

	"github.com/errordeveloper/tape/attest/digest"
	"github.com/errordeveloper/tape/attest/types"
)

const (
	ProviderName = "git"

	DefaultPrimaryRemoteName = "origin"
)

// TODO: need a way to detect multiple repos, for now PathChecker is only meant
// to be used for the manifest dir iteself, and assume there is no nested repos

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
		ObjectHash *string             `json:"objectHash,omitempty"`
		Remotes    map[string][]string `json:"remotes,omitempty"`
		Reference  GitReference        `json:"reference,omitempty"`
	}

	GitReference struct {
		Name   string `json:"name,omitempty"`
		Hash   string `json:"hash,omitempty"`
		Type   string `json:"type,omitempty"`
		Target string `json:"target,omitempty"`
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

	// TODO: determine position of local branch against remote
	// TODO: introduce notion of primary remote branch to determine the possition of the working branch
	// TODO: determine if a tag is used
	// TODO: also check if local tag in sync wirth remote tag
	// TODO: provide info on singed tags/commits

	if summary.Unmodified {
		git.ObjectHash = new(string)
		*git.ObjectHash = c.cache.obj.ID().String()
	} else if c.IsBlob() {
		// there is currently no easy way to obtain a hash for a subtree
		git.ObjectHash = new(string)
		*git.ObjectHash = c.cache.blobHash
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

	head, err := c.cache.repo.Head()
	if err != nil {
		return nil, err
	}

	git.Reference = GitReference{
		Name:   head.Name().String(),
		Hash:   head.Hash().String(),
		Type:   head.Type().String(),
		Target: head.Target().String(),
	}

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
