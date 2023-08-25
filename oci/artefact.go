package oci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"time"

	ociclient "github.com/fluxcd/pkg/oci"
	"github.com/go-git/go-git/v5/utils/ioutil"
	"github.com/google/go-containerregistry/pkg/compression"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	typesv1 "github.com/google/go-containerregistry/pkg/v1/types"

	attestTypes "github.com/docker/labs-brown-tape/attest/types"
	manifestTypes "github.com/docker/labs-brown-tape/manifest/types"
)

const (
	mediaTypePrefix  = "application/vnd.docker.tape"
	ConfigMediaType  = mediaTypePrefix + ".config.v1alpha1+json"
	ContentMediaType = mediaTypePrefix + ".content.v1alpha1.tar+gzip"
	AttestMediaType  = mediaTypePrefix + ".attest.v1alpha1.jsonl+gzip"

	ContentInterpreterAnnotation   = mediaTypePrefix + ".content-interpreter.v1alpha1"
	ContentInterpreterKubectlApply = mediaTypePrefix + ".kubectl-apply.v1alpha1.tar+gzip"

	AttestationsSummaryAnnotations = mediaTypePrefix + ".attestations-summary.v1alpha1"

	// TODO: content interpreter invocation with an image

	regularFileMode = 0o640
)

type ArtefactInfo struct {
	io.ReadCloser

	MediaType   MediaType
	Annotations map[string]string
}

func (c *Client) GetArtefact(ctx context.Context, ref string) (*ArtefactInfo, error) {
	image, err := c.Pull(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to pull %q: %w", ref, err)
	}
	manifest, err := image.Manifest()
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest of %q: %w", ref, err)
	}
	if len(manifest.Layers) < 1 {
		return nil, fmt.Errorf("no layers found in image %q", ref)
	}
	if len(manifest.Layers) > 1 {
		return nil, fmt.Errorf("multiple layers found in image %q", ref)
	}
	layerDecriptor := manifest.Layers[0]

	layer, err := image.LayerByDigest(layerDecriptor.Digest)
	if err != nil {
		return nil, fmt.Errorf("fetching aretefact image failed: %w", err)
	}

	blob, err := layer.Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("extracting uncompressed aretefact image failed: %w", err)
	}

	info := &ArtefactInfo{
		ReadCloser:  blob,
		MediaType:   layerDecriptor.MediaType,
		Annotations: layerDecriptor.Annotations,
	}

	return info, nil
}

// based on https://github.com/fluxcd/pkg/blob/2a323d771e17af02dee2ccbbb9b445b78ab048e5/oci/client/push.go
func (c *Client) PushArtefact(ctx context.Context, destinationRef, sourceDir string, timestamp *time.Time, sourceAttestations ...attestTypes.Statement) (string, error) {
	tmpDir, err := os.MkdirTemp("", "bpt-oci-artefact-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "artefact.tgz")

	outputFile, err := os.OpenFile(tmpFile, os.O_RDWR|os.O_CREATE|os.O_EXCL, regularFileMode)
	if err != nil {
		return "", err
	}
	defer outputFile.Close()

	c.hash.Reset()

	output := io.MultiWriter(outputFile, c.hash)

	if err := c.BuildArtefact(tmpFile, sourceDir, output); err != nil {
		return "", err
	}

	attestLayer, err := c.BuildAttestations(sourceAttestations)
	if err != nil {
		return "", fmt.Errorf("failed to serialise attestations: %w", err)
	}

	ref := destinationRef + ":" + manifestTypes.ConfigImageTagPrefix + hex.EncodeToString(c.hash.Sum(nil))
	tag, err := name.ParseReference(ref)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	if timestamp == nil {
		timestamp = new(time.Time)
		*timestamp = time.Now().UTC()
	}

	indexAnnotations := map[string]string{
		ociclient.CreatedAnnotation: timestamp.Format(time.RFC3339),
	}

	index := mutate.Annotations(
		empty.Index,
		indexAnnotations,
	).(v1.ImageIndex)

	configAnnotations := maps.Clone(indexAnnotations)

	configAnnotations[ContentInterpreterAnnotation] = ContentInterpreterKubectlApply

	config := mutate.Annotations(
		mutate.ConfigMediaType(
			mutate.MediaType(empty.Image, typesv1.OCIManifestSchema1),
			ConfigMediaType,
		),
		configAnnotations,
	).(v1.Image)

	// There is an option to use LayerFromReader which will avoid writing any files to disk,
	// albeit it might impact memory usage and there is no strict security requirement, and
	// manifests do get written out already anyway.
	configLayer, err := tarball.LayerFromFile(tmpFile,
		tarball.WithMediaType(ContentMediaType),
		tarball.WithCompression(compression.GZip),
		tarball.WithCompressedCaching,
	)
	if err != nil {
		return "", fmt.Errorf("creating artefact content layer failed: %w", err)
	}

	config, err = mutate.Append(config, mutate.Addendum{Layer: configLayer})
	if err != nil {
		return "", fmt.Errorf("appeding content to artifact failed: %w", err)
	}

	index = mutate.AppendManifests(index,
		mutate.IndexAddendum{Add: config},
	)

	if attestLayer != nil {
		attestAnnotations := maps.Clone(indexAnnotations)

		summary, err := (attestTypes.Statements)(sourceAttestations).MarshalSummaryAnnotation()
		if err != nil {
			return "", err
		}
		attestAnnotations[AttestationsSummaryAnnotations] = summary

		attest := mutate.Annotations(
			mutate.ConfigMediaType(
				mutate.MediaType(empty.Image, typesv1.OCIManifestSchema1),
				ConfigMediaType,
			),
			attestAnnotations,
		).(v1.Image)

		attest, err = mutate.Append(attest, mutate.Addendum{Layer: attestLayer})
		if err != nil {
			return "", fmt.Errorf("appeding attestations to artifact failed: %w", err)
		}

		index = mutate.AppendManifests(index,
			mutate.IndexAddendum{Add: attest},
		)
	}

	digest, err := index.Digest()
	if err != nil {
		return "", fmt.Errorf("parsing index digest failed: %w", err)
	}

	if err := remote.WriteIndex(tag, index, crane.GetOptions(c.artefactPushOptions(ctx)...).Remote...); err != nil {
		return "", fmt.Errorf("pushing index failed: %w", err)
	}

	return ref + "@" + digest.String(), err
}

func (c *Client) artefactPushOptions(ctx context.Context) []crane.Option {
	return append(c.withContext(ctx),
		crane.WithPlatform(&v1.Platform{
			Architecture: "unknown",
			OS:           "unknown",
		}))
}

// based on https://github.com/fluxcd/pkg/blob/2a323d771e17af02dee2ccbbb9b445b78ab048e5/oci/client/build.go
func (c *Client) BuildArtefact(artifactPath,
	sourceDir string, output io.Writer) error {
	absDir, err := filepath.Abs(sourceDir)
	if err != nil {
		return err
	}

	dirStat, err := os.Stat(absDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("invalid source dir path: %s", absDir)
	}

	gw := gzip.NewWriter(output)
	tw := tar.NewWriter(gw)
	if err := filepath.WalkDir(absDir, func(p string, di os.DirEntry, prevErr error) (err error) {
		if prevErr != nil {
			return prevErr
		}

		// Ignore anything that is not a file or directories e.g. symlinks
		ft := di.Type()
		if !(ft.IsRegular() || ft.IsDir()) {
			return nil
		}

		fi, err := di.Info()
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, p)
		if err != nil {
			return err
		}
		if dirStat.IsDir() {
			// The name needs to be modified to maintain directory structure
			// as tar.FileInfoHeader only has access to the base name of the file.
			// Ref: https://golang.org/src/archive/tar/common.go?#L6264
			//
			// we only want to do this if a directory was passed in
			relFilePath, err := filepath.Rel(absDir, p)
			if err != nil {
				return err
			}
			// Normalize file path so it works on windows
			header.Name = filepath.ToSlash(relFilePath)
		}

		// Remove any environment specific data.
		header.Gid = 0
		header.Uid = 0
		header.Uname = ""
		header.Gname = ""
		header.ModTime = time.Time{}
		header.AccessTime = time.Time{}
		header.ChangeTime = time.Time{}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !ft.IsRegular() {
			return nil
		}

		file, err := os.Open(p)
		if err != nil {
			return err
		}
		defer ioutil.CheckClose(file, &err)

		if _, err := io.Copy(tw, file); err != nil {
			return err
		}
		return nil
	}); err != nil {
		_ = tw.Close()
		_ = gw.Close()
		return err
	}

	if err := tw.Close(); err != nil {
		_ = gw.Close()
		return err
	}
	if err := gw.Close(); err != nil {
		return err
	}

	return nil
}

func (c *Client) BuildAttestations(statements []attestTypes.Statement) (v1.Layer, error) {
	if len(statements) == 0 {
		return nil, nil
	}
	output := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(output)

	if err := attestTypes.Statements(statements).Encode(gw); err != nil {
		return nil, err
	}

	if err := gw.Close(); err != nil {
		return nil, err
	}

	layer, err := tarball.LayerFromOpener(
		func() (io.ReadCloser, error) {
			// this doesn't copy data, it should re-use same undelying slice
			return io.NopCloser(bytes.NewReader(output.Bytes())), nil
		},
		tarball.WithMediaType(AttestMediaType),
		tarball.WithCompression(compression.GZip),
		tarball.WithCompressedCaching,
	)
	if err != nil {
		return nil, fmt.Errorf("creating attestations layer failed: %w", err)
	}

	return layer, nil
}
