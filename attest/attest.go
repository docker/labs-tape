package attest

import (
	"fmt"

	"github.com/errordeveloper/tape/attest/digest"
	"github.com/errordeveloper/tape/attest/types"
	"github.com/errordeveloper/tape/attest/vcs/git"
)

var (
	_ types.PathChecker = (*git.PathChecker)(nil)
)

func DetectVCS(path string) (bool, *PathCheckerRegistry, error) {
	for _, provider := range map[string]func(string, digest.SHA256) types.PathChecker{
		// TODO: support other VCS providers
		git.ProviderName: git.NewPathChecker,
	} {
		checker := provider(path, "")
		ok, err := checker.DetectRepo()
		if err != nil {
			return false, nil, fmt.Errorf("unable to detect VCS: %w", err)
		}
		if ok {
			registry := NewPathCheckerRegistry(path, provider)
			if err := registry.init(checker); err != nil {
				return false, nil, err
			}
			return true, registry, nil
		}
	}
	return false, nil, nil
}
