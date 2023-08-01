package packager

import (
	"context"
	"time"

	"github.com/docker/labs-brown-tape/oci"
)

type Packager interface {
	//Pull(string) error
	Push(context.Context, string) (string, error)
}

type DefaultPackager struct {
	*oci.Client
	destinationRef       string
	sourceEpochTimestamp *time.Time
}

func NewDefaultPackager(client *oci.Client, destinationRef string, sourceEpochTimestamp *time.Time) Packager {
	if client == nil {
		client = oci.NewClient(nil)
	}
	return &DefaultPackager{
		Client:               client,
		destinationRef:       destinationRef,
		sourceEpochTimestamp: sourceEpochTimestamp,
	}
}

func (r *DefaultPackager) Push(ctx context.Context, dir string) (string, error) {
	return r.Client.PushArtefact(ctx, r.destinationRef, dir, r.sourceEpochTimestamp)
}
