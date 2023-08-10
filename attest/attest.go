package attest

import (
	"fmt"

	"github.com/docker/labs-brown-tape/attest/types"
	"github.com/docker/labs-brown-tape/attest/vcs/git"
)

var (
	_ types.PathChecker = (*git.PathChecker)(nil)
)

func DetectVCS(path string) (types.PathChecker, *PathCheckerRegistry, error) {
	for _, provider := range map[string]func(string) types.PathChecker{
		// TODO: support other VCS providers
		git.ProviderName: git.NewPathChecker,
	} {
		checker := provider(path)
		ok, err := checker.DetectRepo()
		if err != nil {
			return nil, nil, fmt.Errorf("unable to detect VCS: %w", err)
		}
		if ok {
			registry := NewPathCheckerRegistry(path, provider)
			if err := registry.Init(); err != nil {
				return nil, nil, err
			}
			return checker, registry, nil
		}
	}
	return nil, nil, nil
}
