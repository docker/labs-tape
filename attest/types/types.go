package types

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"slices"

	toto "github.com/in-toto/in-toto-golang/in_toto"

	"github.com/errordeveloper/tape/attest/digest"
)

type (
	PathCheckerRegistryKey struct {
		Path   string
		Digest digest.SHA256
	}
	Mutations = map[PathCheckerRegistryKey]digest.SHA256

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
		GetPredicate() any
		ExportSubject() []toto.Subject
		Export() toto.Statement
	}
	Statement interface {
		ExportableStatement

		GetSubject() Subjects
		Encode(io.Writer) error
		EncodeWith(EncodeFunc) error
		SetSubjects(func(*Subject) error) error
		Compare(Statement) Cmp
	}

	Predicate[T any] struct {
		Type                   string `json:"predicateType"`
		ComparablePredicate[T] `json:"predicate"`
	}

	ComparablePredicate[T any] interface {
		Compare(T) Cmp
	}

	GenericStatement[T any] struct {
		Subjects Subjects `json:"subject"`
		Predicate[T]
	}

	SummaryAnnotation struct {
		NumStamentes   int      `json:"numStamentes"`
		PredicateTypes []string `json:"predicateTypes"`
		Subjects       Subjects `json:"subject"`
	}
)

var (
	_ Statement = (*GenericStatement[any])(nil)
)

type Cmp *int

func CmpLess() Cmp  { cmp := -1; return &cmp }
func CmpEqual() Cmp { cmp := 0; return &cmp }
func CmpMore() Cmp  { cmp := +1; return &cmp }

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
		Subjects:       make(Subjects, 0, len(subjects)),
	}
	for t := range types {
		summary.PredicateTypes = append(summary.PredicateTypes, t)
	}
	slices.Sort(summary.PredicateTypes)
	for s := range subjects {
		summary.Subjects = append(summary.Subjects, s)
	}
	slices.SortFunc(summary.Subjects, func(a, b Subject) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return summary
}

func (s Statements) MarshalSummaryAnnotation() (string, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 1024))
	base64 := base64.NewEncoder(base64.StdEncoding, buf)
	if err := json.NewEncoder(base64).Encode(s.MakeSummaryAnnotation()); err != nil {
		return "", fmt.Errorf("encoding attestations summary failed: %w", err)
	}
	if err := base64.Close(); err != nil {
		return "", fmt.Errorf("cannot close base64 encoder while encoding attestations summary: %w", err)
	}
	return buf.String(), nil
}

func UnmarshalSummaryAnnotation(s string) (*SummaryAnnotation, error) {
	summary := &SummaryAnnotation{}
	base64 := base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(s))
	if err := json.NewDecoder(base64).Decode(summary); err != nil {
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

func (s Subjects) MarshalJSON() ([]byte, error) { return json.Marshal(s.Export()) }

func (s *Subjects) UnmarshalJSON(data []byte) error {
	subjects := []toto.Subject{}
	if err := json.Unmarshal(data, &subjects); err != nil {
		return err
	}
	if len(subjects) == 0 {
		return fmt.Errorf("invalid subject: zero entries")
	}
	for i := range subjects {
		digestValue, ok := subjects[i].Digest["sha256"]
		if !ok {
			return fmt.Errorf("invalid subject: missing sha256 digest")
		}
		*s = append(*s, Subject{
			Name:   subjects[i].Name,
			Digest: digest.SHA256(digestValue),
		})
	}
	return nil
}

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
	slices.Sort(collection.Providers)

	slices.SortFunc(collection.EntryGroups, comparePathCheckSummariesSlice)
	for g := range collection.EntryGroups {
		if len(collection.EntryGroups[g]) <= 1 {
			continue
		}
		slices.SortFunc(collection.EntryGroups[g][1:], comparePathCheckSummaries)
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

func (a PathCheckSummaryCollection) Compare(b PathCheckSummaryCollection) int {
	if cmp := slices.Compare(a.Providers, b.Providers); cmp != 0 {
		return cmp
	}
	numGroupsA, numGroupsB := len(a.EntryGroups), len(b.EntryGroups)
	if numGroupsA > numGroupsB {
		return +1
	}
	if numGroupsA < numGroupsB {
		return -1
	}
	if numGroupsA == 0 && numGroupsB == 0 {
		return 0
	}

	for g := range a.EntryGroups {
		if cmp := comparePathCheckSummariesSlice(a.EntryGroups[g], b.EntryGroups[g]); cmp != 0 {
			return cmp
		}
	}

	return 0
}

func comparePathCheckSummariesSlice(a, b []PathCheckSummary) int {
	if cmp := cmp.Compare(a[0].ProviderName(), b[0].ProviderName()); cmp != 0 {
		return cmp
	}
	return comparePathCheckSummaries(a[0], b[0])
}

func comparePathCheckSummaries(a, b PathCheckSummary) int {
	return cmp.Compare(a.Common().Path, b.Common().Path)
}

func (p Predicate[T]) GetType() string   { return p.Type }
func (p Predicate[T]) GetPredicate() any { return p.ComparablePredicate }

func (p Predicate[T]) Compare(b any) Cmp {
	if b, ok := b.(Predicate[T]); ok {
		return p.ComparablePredicate.Compare(b.ComparablePredicate.(T))
	}
	return nil
}

func MakeStatement[T any](predicateType string, predicate ComparablePredicate[T], subjects ...Subject) GenericStatement[T] {
	statement := GenericStatement[T]{
		Predicate: Predicate[T]{
			Type:                predicateType,
			ComparablePredicate: predicate,
		},
		Subjects: subjects,
	}
	slices.SortFunc(statement.Subjects, func(a, b Subject) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return statement
}

func (s GenericStatement[T]) GetSubject() Subjects          { return s.Subjects }
func (s GenericStatement[T]) ExportSubject() []toto.Subject { return s.Subjects.Export() }
func (s GenericStatement[T]) Export() toto.Statement        { return Export(s) }
func (s GenericStatement[T]) EncodeWith(e EncodeFunc) error { return e(s.Export()) }

func (s GenericStatement[T]) Encode(w io.Writer) error {
	return s.EncodeWith(json.NewEncoder(w).Encode)
}

func (s *GenericStatement[T]) SetSubjects(f func(subject *Subject) error) error {
	for i := range s.Subjects {
		if err := f(&s.Subjects[i]); err != nil {
			return err
		}
	}
	return nil
}

func (a GenericStatement[T]) Compare(b Statement) Cmp {
	if cmp := cmp.Compare(a.GetType(), b.GetType()); cmp != 0 {
		return &cmp
	}
	subjectsA, subjectsB := a.GetSubject(), b.GetSubject()
	if cmp := cmp.Compare(len(subjectsA), len(subjectsB)); cmp != 0 {
		return &cmp
	}
	if cmp := cmp.Compare(subjectsA[0].Name, subjectsB[0].Name); cmp != 0 {
		return &cmp
	}
	return a.Predicate.Compare(b.GetPredicate())
}
