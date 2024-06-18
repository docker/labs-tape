package manifest

import (
	"cmp"

	"github.com/errordeveloper/tape/attest/types"
)

const (
	ManifestDirPredicateType = "docker.com/tape/ManifestDir/v0.1"
)

var (
	_ types.Statement = (*DirContents)(nil)
)

type DirContents struct {
	types.GenericStatement[SourceDirectory]
}

type SourceDirectory struct {
	Path string `json:"path"`

	VCSEntries *types.PathCheckSummaryCollection `json:"vcsEntries"`
}

func MakeDirContentsStatement(dir string, entries *types.PathCheckSummaryCollection) types.Statement {
	return &DirContents{
		types.MakeStatement[SourceDirectory](
			ManifestDirPredicateType,
			struct {
				SourceDirectory `json:"containedInDirectory"`
			}{
				SourceDirectory{
					Path:       dir,
					VCSEntries: entries,
				},
			},
			entries.Subject()...,
		),
	}
}

func (a SourceDirectory) Compare(b SourceDirectory) types.Cmp {
	if cmp := cmp.Compare(a.Path, b.Path); cmp != 0 {
		return &cmp
	}
	if a.VCSEntries == nil && b.VCSEntries != nil {
		return types.CmpLess()
	}
	if a.VCSEntries != nil && b.VCSEntries == nil {
		return types.CmpMore()
	}
	cmp := a.VCSEntries.Compare(*b.VCSEntries)
	return &cmp
}
