package packager

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"os"

	ociclient "github.com/fluxcd/pkg/oci/client"
	"sigs.k8s.io/kustomize/api/image"

	"github.com/docker/labs-brown-tape/manifest/types"
	"github.com/docker/labs-brown-tape/oci"
)

type Packager interface {
	//Pull(string) error
	Push(context.Context, string) (string, error)
}

type DefaultPackager struct {
	*oci.Client
	DestinationRef string
	hash           hash.Hash
}

func NewDefaultPackager(client *oci.Client, destinationRef string) Packager {
	if client == nil {
		client = oci.NewClient(nil)
	}
	return &DefaultPackager{
		Client:         client,
		DestinationRef: destinationRef,
		hash:           sha256.New(),
	}
}

func (r *DefaultPackager) Push(ctx context.Context, dir string) (string, error) {
	// TODO: this ends up calling Build twice, perhaps push can be split,
	// or use a callback whith an additional writer to set the tag
	tmpFile, err := os.CreateTemp("", "bpt-manifest-packager-*.tgz")
	if err != nil {
		return "", err
	}
	if err := r.Client.Build(tmpFile.Name(), dir, nil); err != nil {
		return "", err
	}
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", err
	}
	if err := os.Remove(tmpFile.Name()); err != nil {
		return "", err
	}

	r.hash.Reset()
	_, _ = r.hash.Write(data)

	ref := r.DestinationRef + ":" + types.ConfigImageTagPrefix + hex.EncodeToString(r.hash.Sum(nil))

	digest, err := r.Client.Push(ctx, ref, dir, ociclient.Metadata{}, nil)
	if err != nil {
		return "", err
	}
	// digest here is in the form <name>@<digest>, we need to include the tag
	_, _, digest = image.Split(digest)
	return ref + "@" + digest, nil
}
