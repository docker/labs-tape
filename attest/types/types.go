package types

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/docker/labs-brown-tape/attest/digest"

	toto "github.com/in-toto/in-toto-golang/in_toto"
)

type PathChecker interface {
	ProviderName() string
	DetectRepo() (bool, error)
	Check() (bool, bool, error)
	MakeSummary() (PathCheckSummary, error)
}

type (
	PathCheckSummaryCommon struct {
		Unmodified bool   `json:"unmodified"`
		Path       string `json:"path,omitempty"`
		URI        string `json:"uri,omitempty"`
	}

	PathCheckSummary interface {
		Common() PathCheckSummaryCommon
		Full() interface{}
	}

	Subject struct {
		Name   string        `json:"name"`
		Digest digest.SHA256 `json:"digest"`
	}

	Statements []Statement
)

func (s PathCheckSummaryCommon) Common() PathCheckSummaryCommon { return s }

type (
	EncodeFunc func(any) error
	Statement  interface {
		Type() string
		Data() interface{}
		GetSubjectName() string
		GetSubjectDigest() digest.SHA256
		ExportSubject() []toto.Subject
		Export() toto.Statement
		Encode(io.Writer) error
		EncodeWith(EncodeFunc) error
	}
)

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
