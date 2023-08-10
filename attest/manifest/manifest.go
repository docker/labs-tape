package manifest

import (
	"github.com/docker/labs-brown-tape/attest/types"
)

const (
	ManifestDirPredicateType = "docker.com/tape/ManifestDir/v0.1"
)

var (
	_ types.Statement = (*DirContents)(nil)
)

type DirContents struct{ types.MultiSubjectStatement }

type SourceDirectory struct {
	Path string `json:"path"`

	VCSEntries *types.PathCheckSummaryCollection `json:"vcsEntries"`
}

func MakeDirContentsStatement(dir string, entries *types.PathCheckSummaryCollection) types.Statement {
	return &DirContents{
		types.MakeMultiSubjectStatement(
			entries.Subject(),
			ManifestDirPredicateType,
			struct {
				ContainedInDirectory SourceDirectory `json:"containedInDirectory"`
			}{
				SourceDirectory{
					Path:       dir,
					VCSEntries: entries,
				},
			},
		),
	}
}
