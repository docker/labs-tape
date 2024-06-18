package packager

import (
	"context"
	"time"

	attestTypes "github.com/errordeveloper/tape/attest/types"
	"github.com/errordeveloper/tape/oci"
)

type Packager interface {
	//Pull(string) error
	Push(context.Context, string) (string, error)
}

type DefaultPackager struct {
	*oci.Client
	destinationRef       string
	sourceEpochTimestamp *time.Time
	sourceAttestations   attestTypes.Statements
}

func NewDefaultPackager(client *oci.Client, destinationRef string, sourceEpochTimestamp *time.Time, sourceAttestations ...attestTypes.Statement) Packager {
	if client == nil {
		client = oci.NewClient(nil)
	}
	return &DefaultPackager{
		Client:               client,
		destinationRef:       destinationRef,
		sourceEpochTimestamp: sourceEpochTimestamp,
		sourceAttestations:   sourceAttestations,
	}
}

func (r *DefaultPackager) Push(ctx context.Context, dir string) (string, error) {
	return r.Client.PushArtefact(ctx, r.destinationRef, dir,
		r.sourceEpochTimestamp, r.sourceAttestations...)
}
