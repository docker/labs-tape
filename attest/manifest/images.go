package manifest

import (
	"cmp"

	attestTypes "github.com/errordeveloper/tape/attest/types"
	manifestTypes "github.com/errordeveloper/tape/manifest/types"
)

const (
	OriginalImageRefPredicateType = "docker.com/tape/OriginalImageRef/v0.1"
	ResolvedImageRefPredicateType = "docker.com/tape/ResolvedImageRef/v0.1"
	ReplacedImageRefPredicateType = "docker.com/tape/ReplacedImageRef/v0.1"
)

var (
	_ attestTypes.Statement = (*OriginalImageRef)(nil)
	_ attestTypes.Statement = (*ResolvedImageRef)(nil)
)

type OriginalImageRef struct {
	attestTypes.GenericStatement[ImageRefenceWithLocation]
}

type ResolvedImageRef struct {
	attestTypes.GenericStatement[ImageRefenceWithLocation]
}

type ReplacedImageRef struct {
	attestTypes.GenericStatement[ImageRefenceWithLocation]
}

type ImageRefenceWithLocation struct {
	Reference string  `json:"reference"`
	Line      int     `json:"line"`
	Column    int     `json:"column"`
	Alias     *string `json:"alias,omitempty"`
}

// TODO:
// - related tags (just the tags)
// - copy inline atteststations, and reference them
// - copy sigstore attestations, and reference them

func MakeOriginalImageRefStatements(images *manifestTypes.ImageList) attestTypes.Statements {
	statements := attestTypes.Statements{}
	forEachImage(images, func(subject attestTypes.Subject, ref ImageRefenceWithLocation) {
		s := &OriginalImageRef{
			attestTypes.MakeStatement(
				OriginalImageRefPredicateType,
				struct {
					ImageRefenceWithLocation `json:"foundImageReference"`
				}{ref},
				subject,
			),
		}
		statements = append(statements, s)
	})
	return statements
}

func MakeReplacedImageRefStatements(images *manifestTypes.ImageList) attestTypes.Statements {
	statements := attestTypes.Statements{}
	forEachImage(images, func(subject attestTypes.Subject, ref ImageRefenceWithLocation) {
		statements = append(statements, &ReplacedImageRef{
			attestTypes.MakeStatement(
				ReplacedImageRefPredicateType,
				struct {
					ImageRefenceWithLocation `json:"replacedImageReference"`
				}{ref},
				subject,
			),
		})
	})
	return statements
}

func MakeResovedImageRefStatements(images *manifestTypes.ImageList) attestTypes.Statements {
	statements := attestTypes.Statements{}
	forEachImage(images, func(subject attestTypes.Subject, ref ImageRefenceWithLocation) {
		statements = append(statements, &ResolvedImageRef{
			attestTypes.MakeStatement(
				ResolvedImageRefPredicateType,
				struct {
					ImageRefenceWithLocation `json:"resolvedImageReference"`
				}{ref},
				subject,
			),
		})
	})
	return statements
}

func forEachImage(images *manifestTypes.ImageList, do func(attestTypes.Subject, ImageRefenceWithLocation)) {
	for _, image := range images.Items() {
		for _, source := range image.Sources {
			do(
				attestTypes.Subject{
					Name:   source.Manifest,
					Digest: source.ManifestDigest,
				},
				ImageRefenceWithLocation{
					Reference: image.Ref(true),
					Line:      source.Line,
					Column:    source.Column,
					Alias:     image.Alias,
				},
			)
		}
	}
}

func (a ImageRefenceWithLocation) Compare(b ImageRefenceWithLocation) attestTypes.Cmp {
	if cmp := cmp.Compare(a.Reference, b.Reference); cmp != 0 {
		return &cmp
	}
	if cmp := cmp.Compare(a.Line, b.Line); cmp != 0 {
		return &cmp
	}
	if cmp := cmp.Compare(a.Column, b.Column); cmp != 0 {
		return &cmp
	}
	return attestTypes.CmpEqual()
}
