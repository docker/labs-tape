package oci

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	ociclient "github.com/fluxcd/pkg/oci"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	typesv1 "github.com/google/go-containerregistry/pkg/v1/types"

	attestTypes "github.com/docker/labs-brown-tape/attest/types"
	manifestTypes "github.com/docker/labs-brown-tape/manifest/types"
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

	if err := c.BuildArtefact(tmpFile, sourceDir); err != nil {
		return "", err
	}

	// TODO: can avoid re-reading the file by rewrtiting the Build function and passing io.TeeWriter
	data, err := os.OpenFile(tmpFile, os.O_RDONLY, 0)
	if err != nil {
		return "", err
	}

	c.hash.Reset()
	if _, err := io.Copy(c.hash, data); err != nil {
		return "", err
	}

	ref := destinationRef + ":" + manifestTypes.ConfigImageTagPrefix + hex.EncodeToString(c.hash.Sum(nil))
	// _, err := name.ParseReference(ref)
	// if err != nil {
	// 	return "", fmt.Errorf("invalid URL: %w", err)
	// }

	if timestamp == nil {
		timestamp = new(time.Time)
		*timestamp = time.Now().UTC()
	}

	img := mutate.Annotations(
		mutate.ConfigMediaType(
			// TODO: define tape media types
			mutate.MediaType(empty.Image, typesv1.OCIManifestSchema1),
			ociclient.CanonicalConfigMediaType,
		),
		map[string]string{
			ociclient.CreatedAnnotation: timestamp.Format(time.RFC3339),
		},
	).(v1.Image)

	layer, err := tarball.LayerFromFile(tmpFile, tarball.WithMediaType(ociclient.CanonicalContentMediaType))
	if err != nil {
		return "", fmt.Errorf("creating content layer failed: %w", err)
	}

	img, err = mutate.Append(img, mutate.Addendum{Layer: layer})
	if err != nil {
		return "", fmt.Errorf("appeding content to artifact failed: %w", err)
	}

	if err := crane.Push(img, ref, c.artefactPushOptions(ctx)...); err != nil {
		return "", fmt.Errorf("pushing artifact failed: %w", err)
	}

	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("parsing artifact digest failed: %w", err)
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
func (c *Client) BuildArtefact(artifactPath, sourceDir string) (err error) {
	absDir, err := filepath.Abs(sourceDir)
	if err != nil {
		return err
	}

	dirStat, err := os.Stat(absDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("invalid source dir path: %s", absDir)
	}

	tf, err := os.CreateTemp(filepath.Split(absDir))
	if err != nil {
		return err
	}
	tmpName := tf.Name()
	defer func() {
		if err != nil {
			os.Remove(tmpName)
		}
	}()

	sz := &writeCounter{}
	mw := io.MultiWriter(tf, sz)

	gw := gzip.NewWriter(mw)
	tw := tar.NewWriter(gw)
	if err := filepath.Walk(absDir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Ignore anything that is not a file or directories e.g. symlinks
		if m := fi.Mode(); !(m.IsRegular() || m.IsDir()) {
			return nil
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

		if !fi.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			f.Close()
			return err
		}
		if _, err := io.Copy(tw, f); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}); err != nil {
		tw.Close()
		gw.Close()
		tf.Close()
		return err
	}

	if err := tw.Close(); err != nil {
		gw.Close()
		tf.Close()
		return err
	}
	if err := gw.Close(); err != nil {
		tf.Close()
		return err
	}
	if err := tf.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tmpName, 0o640); err != nil {
		return err
	}

	return fs.RenameWithFallback(tmpName, artifactPath)
}

type writeCounter struct {
	written int64
}

func (wc *writeCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.written += int64(n)
	return n, nil
}
