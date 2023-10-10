package manifest

import (
	"cmp"

	"github.com/docker/labs-brown-tape/attest/types"
	attestTypes "github.com/docker/labs-brown-tape/attest/types"
)

const (
	ManifestDirPredicateType = "docker.com/tape/ManifestDir/v0.2"
)

var (
	_ types.Statement = (*DirContents)(nil)
)

type DirContents struct {
	types.GenericStatement[SourceDirectoryContents]
}

type SourceDirectory struct {
	Path string `json:"path"`

	VCSEntries *types.PathCheckSummaryCollection `json:"vcsEntries"`
}

type SourceDirectoryContents struct {
	SourceDirectory `json:"containedInDirectory"`
}

func MakeDirContentsStatement(dir string, entries *types.PathCheckSummaryCollection) types.Statement {
	return &DirContents{
		types.MakeStatement[SourceDirectoryContents](
			ManifestDirPredicateType,
			SourceDirectoryContents{
				SourceDirectory: SourceDirectory{
					Path:       dir,
					VCSEntries: entries,
				},
			},
			entries.Subject()...,
		),
	}
}

func MakeDirContentsStatementFrom(statement types.Statement) DirContents {
	dirContents := DirContents{
		GenericStatement: attestTypes.GenericStatement[SourceDirectoryContents]{},
	}
	dirContents.ConvertFrom(statement)
	return dirContents
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

func (a SourceDirectoryContents) Compare(b SourceDirectoryContents) types.Cmp {
	return a.SourceDirectory.Compare(b.SourceDirectory)
}
