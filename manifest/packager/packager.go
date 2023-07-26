package packager

import (
	"context"

	"github.com/docker/labs-brown-tape/oci"
)

type Packager interface {
	//Pull(string) error
	Push(context.Context, string) (string, error)
}

type DefaultPackager struct {
	*oci.Client
	DestinationRef string
}

func NewDefaultPackager(client *oci.Client, destinationRef string) Packager {
	if client == nil {
		client = oci.NewClient(nil)
	}
	return &DefaultPackager{
		Client:         client,
		DestinationRef: destinationRef,
	}
}

func (r *DefaultPackager) Push(ctx context.Context, dir string) (string, error) {
	return r.Client.PushArtefact(ctx, r.DestinationRef, dir)
}
