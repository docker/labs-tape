package manifest

import (
	attestTypes "github.com/docker/labs-brown-tape/attest/types"
	manifestTypes "github.com/docker/labs-brown-tape/manifest/types"
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
	attestTypes.SingleSubjectStatement
}

type ResolvedImageRef struct {
	attestTypes.SingleSubjectStatement
}

type ImageRefenceWithLocation struct {
	Reference string `json:"reference"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
}

// TODO:
// - replaced
// - related tags (just the tags)
// - copy inline atteststations, and reference them
// - copy sigstore attestations, and reference them

func MakeOriginalImageRefStatements(images *manifestTypes.ImageList) attestTypes.Statements {
	statements := attestTypes.Statements{}
	forEachImage(images, func(subject attestTypes.Subject, ref ImageRefenceWithLocation) {
		s := &OriginalImageRef{
			attestTypes.MakeSingleSubjectStatement(
				subject,
				OriginalImageRefPredicateType,
				struct {
					FoundImageReference ImageRefenceWithLocation `json:"foundImageReference"`
				}{ref},
			),
		}
		statements = append(statements, s)
	})
	return statements
}

func MakeResovedImageRefStatements(images *manifestTypes.ImageList) attestTypes.Statements {
	statements := attestTypes.Statements{}
	forEachImage(images, func(subject attestTypes.Subject, ref ImageRefenceWithLocation) {
		statements = append(statements, &ResolvedImageRef{
			attestTypes.MakeSingleSubjectStatement(
				subject,
				ResolvedImageRefPredicateType,
				struct {
					ResolvedImageReference ImageRefenceWithLocation `json:"resolvedImageReference"`
				}{ref},
			),
		})
	})
	return statements
}

func forEachImage(images *manifestTypes.ImageList, do func(attestTypes.Subject, ImageRefenceWithLocation)) {
	for _, image := range images.Items() {
		// TODO: drop image.Source and always use image.Sources instead
		sources := image.Sources
		if sources == nil && image.Source != nil {
			sources = []manifestTypes.Source{*image.Source}
		}
		for _, source := range sources {
			do(
				attestTypes.Subject{
					Name:   source.Manifest,
					Digest: source.ManifestDigest,
				},
				ImageRefenceWithLocation{
					Reference: image.Ref(true),
					Line:      source.Line,
					Column:    source.Column,
				},
			)
		}
	}
}
