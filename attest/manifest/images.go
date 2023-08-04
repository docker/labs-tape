package manifest

import (
	"encoding/json"
	"io"

	toto "github.com/in-toto/in-toto-golang/in_toto"

	"github.com/docker/labs-brown-tape/attest/digest"
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
	attestTypes.Subject `json:"subject"`

	FoundImageReference ImageRefenceWithLocation `json:"foundImageReference"`
}

type ResolvedImageRef struct {
	attestTypes.Subject `json:"subject"`

	ResolvedImageReference ImageRefenceWithLocation `json:"resolvedImageReference"`
}

type ImageRefenceWithLocation struct {
	Reference string `json:"reference"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
}

func MakeOriginalImageRefStatements(images *manifestTypes.ImageList) attestTypes.Statements {
	statements := attestTypes.Statements{}
	forEachImage(images, func(subject attestTypes.Subject, ref ImageRefenceWithLocation) {
		statements = append(statements, &OriginalImageRef{
			Subject:             subject,
			FoundImageReference: ref,
		})
	})
	return statements
}

func MakeResovedImageRefStatements(images *manifestTypes.ImageList) attestTypes.Statements {
	statements := attestTypes.Statements{}
	forEachImage(images, func(subject attestTypes.Subject, ref ImageRefenceWithLocation) {
		statements = append(statements, &ResolvedImageRef{
			Subject:                subject,
			ResolvedImageReference: ref,
		})
	})
	return statements
}

func (OriginalImageRef) Type() string                                  { return OriginalImageRefPredicateType }
func (ref OriginalImageRef) Data() interface{}                         { return ref }
func (ref OriginalImageRef) GetSubjectName() string                    { return ref.Subject.Name }
func (ref OriginalImageRef) GetSubjectDigest() digest.SHA256           { return ref.Subject.Digest }
func (ref OriginalImageRef) ExportSubject() []toto.Subject             { return ref.Subject.Export() }
func (ref OriginalImageRef) Export() toto.Statement                    { return export(ref) }
func (ref OriginalImageRef) EncodeWith(e attestTypes.EncodeFunc) error { return e(ref.Export()) }

func (ref OriginalImageRef) Encode(w io.Writer) error {
	return ref.EncodeWith(json.NewEncoder(w).Encode)
}

func (ResolvedImageRef) Type() string                                  { return ResolvedImageRefPredicateType }
func (ref ResolvedImageRef) Data() interface{}                         { return ref }
func (ref ResolvedImageRef) GetSubjectName() string                    { return ref.Subject.Name }
func (ref ResolvedImageRef) GetSubjectDigest() digest.SHA256           { return ref.Subject.Digest }
func (ref ResolvedImageRef) ExportSubject() []toto.Subject             { return ref.Subject.Export() }
func (ref ResolvedImageRef) Export() toto.Statement                    { return export(ref) }
func (ref ResolvedImageRef) EncodeWith(e attestTypes.EncodeFunc) error { return e(ref.Export()) }

func (ref ResolvedImageRef) Encode(w io.Writer) error {
	return ref.EncodeWith(json.NewEncoder(w).Encode)
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

func export(ref attestTypes.Statement) toto.Statement {
	return toto.Statement{
		StatementHeader: toto.StatementHeader{
			Type:          toto.StatementInTotoV01,
			PredicateType: ref.Type(),
			Subject:       ref.ExportSubject(),
		},
		Predicate: ref,
	}
}
