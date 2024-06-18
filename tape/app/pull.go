package app

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/errordeveloper/tape/oci"
	"github.com/fluxcd/pkg/tar"
)

type TapePullCommand struct {
	tape *TapeCommand
	OutputManifestDirOptions

	Image        string `short:"I" long:"image" description:"Name of the image to pull" required:"true"`
	Attestations string `short:"a" long:"attestations" description:"Path to wrtie attestations file"`
}

const regularFileMode = 0o640

func (c *TapePullCommand) Execute(args []string) error {
	ctx := context.WithValue(c.tape.ctx, "command", "pull")
	if len(args) != 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}

	if err := c.tape.Init(); err != nil {
		return err
	}

	client := oci.NewClient(nil)

	artefacts, err := client.Fetch(ctx, c.Image, oci.ContentMediaType, oci.AttestMediaType)
	if err != nil {
		return err
	}

	for i := range artefacts {
		artefact := artefacts[i]
		switch artefact.MediaType {
		case oci.ContentMediaType:
			if err := tar.Untar(artefact, c.OutputManifestDirOptions.ManifestDir, tar.WithMaxUntarSize(-1)); err != nil {
				return fmt.Errorf("failed to exatract manifests: %w", err)
			}
			c.tape.log.Infof("extracted manifest to %q", c.OutputManifestDirOptions.ManifestDir)
			// TODO: add mode to just dump the tarball
		case oci.AttestMediaType:
			if c.Attestations == "" {
				break
			}

			r, w := io.ReadCloser(nil), io.WriteCloser(nil)

			if filepath.Ext(c.Attestations) == ".gz" {
				r = artefact
			} else {
				r, err = gzip.NewReader(artefact)
				if err != nil {
					return fmt.Errorf("failed to decompress attestations file: %w", err)
				}
				defer r.Close()
			}

			if c.Attestations == "-" {
				w = os.Stdout
			} else {
				w, err = os.OpenFile(c.Attestations, os.O_RDWR|os.O_CREATE|os.O_EXCL, regularFileMode)
				if err != nil {
					return fmt.Errorf("failed to create attestations file: %w", err)
				}
				defer w.Close()
			}

			if _, err := io.Copy(w, r); err != nil {
				return fmt.Errorf("failed to write attestations file: %w", err)
			}
			c.tape.log.Infof("extracted attestations to %q", c.Attestations)
		}

	}
	return nil
}
