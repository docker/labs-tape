package types

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"github.com/docker/labs-brown-tape/attest/digest"

	toto "github.com/in-toto/in-toto-golang/in_toto"
)

type (
	PathCheckerRegistryKey struct {
		Path   string
		Digest digest.SHA256
	}

	PathChecker interface {
		ProviderName() string
		DetectRepo() (bool, error)
		Check() (bool, bool, error)
		MakeSummary() (PathCheckSummary, error)
	}

	PathCheckSummaryCommon struct {
		Unmodified bool          `json:"unmodified"`
		Path       string        `json:"path,omitempty"`
		URI        string        `json:"uri,omitempty"`
		IsDir      bool          `json:"isDir,omitempty"`
		Digest     digest.SHA256 `json:"digest,omitempty"`
	}

	PathCheckSummaryCollection struct {
		Providers   []string             `json:"providers"`
		EntryGroups [][]PathCheckSummary `json:"entryGroups"`
	}

	PathCheckSummary interface {
		ProviderName() string
		Common() PathCheckSummaryCommon
		Full() interface{}
		SameRepo(PathCheckSummary) bool
	}

	Subject struct {
		Name   string        `json:"name"`
		Digest digest.SHA256 `json:"digest"`
	}
	Subjects []Subject

	Statements []Statement
)

func (s PathCheckSummaryCommon) Common() PathCheckSummaryCommon { return s }

type (
	EncodeFunc          func(any) error
	ExportableStatement interface {
		GetType() string
		GetPredicate() interface{}
		ExportSubject() []toto.Subject
		Export() toto.Statement
	}
	Statement interface {
		ExportableStatement

		GetSubject() Subjects
		Encode(io.Writer) error
		EncodeWith(EncodeFunc) error
		SetSubjects(func(*Subject) error) error
	}

	Predicate struct {
		Type      string      `json:"predicateType"`
		Predicate interface{} `json:"predicate"`
	}
	SingleSubjectStatement struct {
		Subject Subject `json:"subject"`
		Predicate
	}
	MultiSubjectStatement struct {
		Subjects Subjects `json:"subject"`
		Predicate
	}

	SummaryAnnotation struct {
		NumStamentes   int      `json:"numStamentes"`
		PredicateTypes []string `json:"predicateTypes"`
		Subjects       Subjects `json:"subject"`
	}
)

var (
	_ Statement = (*SingleSubjectStatement)(nil)
	_ Statement = (*MultiSubjectStatement)(nil)
)

func Export(s ExportableStatement) toto.Statement {
	return toto.Statement{
		StatementHeader: toto.StatementHeader{
			Type:          toto.StatementInTotoV01,
			PredicateType: s.GetType(),
			Subject:       s.ExportSubject(),
		},
		Predicate: s.GetPredicate(),
	}
}

func (s Statements) Export() []toto.Statement {
	statements := make([]toto.Statement, len(s))
	for i := range s {
		statements[i] = s[i].Export()
	}
	return statements
}

func (s Statements) EncodeWith(encoder EncodeFunc) error {
	for i := range s {
		if err := s[i].EncodeWith(encoder); err != nil {
			return err
		}
	}
	return nil
}

func (s Statements) Encode(w io.Writer) error {
	return s.EncodeWith(json.NewEncoder(w).Encode)
}

func (s Statements) MakeSummaryAnnotation() SummaryAnnotation {
	types := map[string]struct{}{}
	subjects := map[Subject]struct{}{}
	for _, statement := range s {
		types[statement.GetType()] = struct{}{}
		for _, subject := range statement.GetSubject() {
			subjects[subject] = struct{}{}
		}
	}

	summary := SummaryAnnotation{
		NumStamentes:   len(s),
		PredicateTypes: make([]string, 0, len(types)),
		Subjects:       make(Subjects, len(subjects)),
	}
	for t := range types {
		summary.PredicateTypes = append(summary.PredicateTypes, t)
	}
	for s := range subjects {
		summary.Subjects = append(summary.Subjects, s)
	}

	return summary
}

func (s Statements) MarshalSummaryAnnotation() (string, error) {
	buf := bytes.NewBuffer(nil)
	if err := json.NewEncoder(base64.NewEncoder(base64.StdEncoding, buf)).
		Encode(s.MakeSummaryAnnotation()); err != nil {
		return "", fmt.Errorf("encoding attestations summary failed: %w", err)
	}
	return buf.String(), nil
}

func UnmarshalSummaryAnnotation(s string) (*SummaryAnnotation, error) {
	summary := &SummaryAnnotation{}
	if err := json.NewDecoder(base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(s))).
		Decode(summary); err != nil {
		return nil, fmt.Errorf("decoding attestation summary failed: %w", err)
	}
	return summary, nil
}

func MakeSubject(name string, digest digest.SHA256) Subject { return Subject{name, digest} }
func (s Subject) GetSubjectName() string                    { return s.Name }
func (s Subject) GetSubjectDigest() digest.SHA256           { return s.Digest }

func (s Subject) Export() []toto.Subject {
	return []toto.Subject{{
		Name:   s.Name,
		Digest: s.Digest.DigestSet(),
	}}
}

func (s Subject) MarshalJSON() ([]byte, error) { return json.Marshal(s.Export()) }

func (s *Subject) UnmarshalJSON(data []byte) error {
	subjects := []toto.Subject{}
	if err := json.Unmarshal(data, &subjects); err != nil {
		return err
	}
	switch numSubjects := len(subjects); {
	case numSubjects == 0:
		return fmt.Errorf("invalid subject: zero entries")
	case numSubjects > 1:
		return fmt.Errorf("invalid subject: multiple entries")
	}
	digestValue, ok := subjects[0].Digest["sha256"]
	if !ok {
		return fmt.Errorf("invalid subject: missing sha256 digest")
	}

	*s = Subject{
		Name:   subjects[0].Name,
		Digest: digest.SHA256(digestValue),
	}

	return nil
}

func MakeSubjects(subjects ...Subject) Subjects { return append(Subjects{}, subjects...) }

func (s Subjects) Export() []toto.Subject {
	subjects := make([]toto.Subject, 0, len(s))
	for i := range s {
		subjects = append(subjects, s[i].Export()...)
	}
	return subjects
}

func (s Subjects) MarshalJSON() ([]byte, error)     { return json.Marshal(s.Export()) }
func (s *Subjects) UnmarshalJSON(data []byte) error { return json.Unmarshal(data, s) }

func MakePathCheckSummaryCollection(entries ...PathChecker) (*PathCheckSummaryCollection, error) {
	numEntries := len(entries)
	if numEntries == 0 {
		return nil, nil
	}
	summaries := make([]PathCheckSummary, numEntries)
	for i := range entries {
		if entries[i] == nil {
			return nil, fmt.Errorf("nil entry at index %d", i)
		}
		summary, err := entries[i].MakeSummary()
		if err != nil {
			return nil, err
		}
		summaries[i] = summary
	}

	groups := [][]PathCheckSummary{}
	for s := range summaries {
		matched := false
		for g := range groups {
			if len(groups[g]) == 0 {
				return nil, fmt.Errorf("empty group at index %d", g)
			}
			if summaries[s].SameRepo(groups[g][0]) {
				groups[g] = append(groups[g], summaries[s])
				matched = true
				break
			}
		}
		if !matched {
			groups = append(groups, []PathCheckSummary{summaries[s]})
		}
	}

	providers := map[string]struct{}{}
	for _, entry := range groups {
		providers[entry[0].ProviderName()] = struct{}{}
	}

	collection := &PathCheckSummaryCollection{
		Providers:   make([]string, 0, len(providers)),
		EntryGroups: groups,
	}

	for provider := range providers {
		collection.Providers = append(collection.Providers, provider)
	}
	return collection, nil
}

func (c PathCheckSummaryCollection) Subject() Subjects {
	subject := Subjects{}
	for i := range c.EntryGroups {
		for j := range c.EntryGroups[i] {
			common := c.EntryGroups[i][j].Common()
			if common.Digest == "" {
				continue
			}
			subject = append(subject, MakeSubject(common.Path, common.Digest))
		}
	}
	return subject
}

func (p Predicate) GetType() string           { return p.Type }
func (p Predicate) GetPredicate() interface{} { return p.Predicate }

func MakeSingleSubjectStatement(subject Subject, predicateType string, predicate interface{}) SingleSubjectStatement {
	return SingleSubjectStatement{
		Predicate: Predicate{
			Type:      predicateType,
			Predicate: predicate,
		},
		Subject: subject,
	}
}

func (s SingleSubjectStatement) GetSubject() Subjects          { return MakeSubjects(s.Subject) }
func (s SingleSubjectStatement) ExportSubject() []toto.Subject { return s.Subject.Export() }
func (s SingleSubjectStatement) Export() toto.Statement        { return Export(s) }
func (s SingleSubjectStatement) EncodeWith(e EncodeFunc) error { return e(s.Export()) }

func (s SingleSubjectStatement) Encode(w io.Writer) error {
	return s.EncodeWith(json.NewEncoder(w).Encode)
}

func (s *SingleSubjectStatement) SetSubjects(f func(subject *Subject) error) error {
	return f(&s.Subject)
}

func MakeMultiSubjectStatement(subjects Subjects, predicateType string, predicate interface{}) MultiSubjectStatement {
	return MultiSubjectStatement{
		Predicate: Predicate{
			Type:      predicateType,
			Predicate: predicate,
		},
		Subjects: subjects,
	}
}

func (s MultiSubjectStatement) GetSubject() Subjects          { return s.Subjects }
func (s MultiSubjectStatement) ExportSubject() []toto.Subject { return s.Subjects.Export() }
func (s MultiSubjectStatement) Export() toto.Statement        { return Export(s) }
func (s MultiSubjectStatement) EncodeWith(e EncodeFunc) error { return e(s.Export()) }

func (s MultiSubjectStatement) Encode(w io.Writer) error {
	return s.EncodeWith(json.NewEncoder(w).Encode)
}

func (s *MultiSubjectStatement) SetSubjects(f func(subject *Subject) error) error {
	for i := range s.Subjects {
		if err := f(&s.Subjects[i]); err != nil {
			return err
		}
	}
	return nil
}
