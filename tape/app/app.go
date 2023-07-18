package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	flags "github.com/thought-machine/go-flags"

	"github.com/docker/staples/pkg/logger"

	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/sigstore/sigstore/pkg/cryptoutils"

	"github.com/docker/labs-brown-tape/manifest/imageresolver"
	"github.com/docker/labs-brown-tape/manifest/imagescanner"
	"github.com/docker/labs-brown-tape/manifest/loader"
	"github.com/docker/labs-brown-tape/manifest/types"
	"github.com/docker/labs-brown-tape/oci"
)

type OutputFormat string

const (
	OutputFormatDetailedText OutputFormat = "detailed-text"
	OutputFormatText         OutputFormat = "text"
	OutputFormatDirectJSON   OutputFormat = "direct-json"
)

type TapeCommand struct {
	LogLevel     string       `long:"log-level" description:"Log level" default:"info"`
	OutputFormat OutputFormat `long:"output-format" description:"Format of the output to use" default:"detailed-text"`

	log *logger.Logger
	ctx context.Context
}

type CommonOptions struct {
	ManifestDir string `short:"d" long:"manifest-dir" description:"directory containing manifests" required:"true"`

	tape *TapeCommand
}

type TapeImagesCommand struct {
	CommonOptions
}

type TapeTrustCommand struct {
	CommonOptions
}

func Run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	tape := &TapeCommand{
		log: logger.New(),
		ctx: ctx,
	}

	fp := flags.NewParser(tape, flags.HelpFlag)

	commands := []struct {
		name    string
		short   string
		long    []string
		options interface{}
	}{
		{
			name:  "images",
			short: "List app image",
			long: []string{
				"This command can be used to obeseve app images referenced",
				"in the given set of manifests, inspect metadata and digests",
			},
			options: &TapeImagesCommand{
				CommonOptions: CommonOptions{tape: tape},
			},
		},
	}

	for _, c := range commands {
		_, err := fp.AddCommand(c.name, c.short, strings.Join(c.long, "\n"), c.options)
		if err != nil {
			tape.log.Errorf("failed to add %s command: %s", c.name, err)
			return 1
		}
	}

	if _, err := fp.Parse(); err != nil {
		switch flagsErr := err.(type) {
		case flags.ErrorType:
			if flagsErr == flags.ErrHelp {
				return 0
			}
			tape.log.Errorf("failed to parse flags: %s", err)
			return 1
		default:
			tape.log.Errorf("command failed: %s", err)
			return 1
		}
	}

	return 0
}

func (c *TapeCommand) Init() error {
	if c.log == nil {
		c.log = logger.New()
	}
	if err := c.log.SetLevel(c.LogLevel); err != nil {
		return err
	}
	return nil
}

func (c *TapeImagesCommand) Execute(args []string) error {
	ctx := context.WithValue(c.tape.ctx, "command", "images")
	if len(args) != 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}

	if err := c.tape.Init(); err != nil {
		return err
	}

	loader := loader.NewRecursiveManifestDirectoryLoader(c.ManifestDir)
	if err := loader.Load(); err != nil {
		return fmt.Errorf("failed to load manifests: %w", err)
	}
	c.tape.log.Debugf("loaded manifests: %v", loader.Paths())

	scanner := imagescanner.NewDefaultImageScanner()

	if err := scanner.Scan(loader.RelPaths()); err != nil {
		return fmt.Errorf("failed to scan images: %w", err)
	}

	images := scanner.GetImages()
	c.tape.log.Debugf("found images: %#v", images.Items())

	images.Dedup()

	client := oci.NewClient(nil)
	// TODO: use client.LoginWithCredentials() and/or other options
	// TODO: integrate with docker-credential-helpers

	type imageManifest struct {
		Digest      oci.Hash
		MediaType   oci.MediaType
		Platform    *oci.Platform
		Annotations map[string]string
	}
	type document struct {
		MediaType string
		Data      []byte
		Object    any
	}
	type imageInfo struct {
		Ref            string
		DigestProvided bool
		Sources        []types.Source
		InlineAttestations,
		ExternalAttestations,
		InlineSBOMs, // TODO: implement
		ExternalSBOMs,
		InlineSignatures, // TODO: implement
		ExternalSignatures map[string]document
		Related             map[string][]oci.Metadata
		RelatedUnclassified []string
		Manifests           []imageManifest
	}

	outputInfo := make(map[string]imageInfo, len(images.Items()))

	withDigests := map[string]struct{}{}
	for _, image := range images.Items() {
		if image.Digest != "" {
			withDigests[image.Digest] = struct{}{}
		}
	}

	c.tape.log.Info("resolving image digests")

	if err := imageresolver.NewRegistryResolver(client).ResolveDigests(ctx, images); err != nil {
		return fmt.Errorf("failed to resolve image digests: %w", err)
	}

	// TODO: separate business logic from presentation
	// TODO: avoid calling client.ListRelated() over and over for the same image
	// TODO: simple JSON formatter, as well as attestation formatter
	// TODO: proper table/tree formatter
	// TODO: include info about signatures
	// TODO: include references to manifests where image is used
	// TODO: attempt call `cosign download signature` and find out who signed the image
	// TODO: get OCI annotations, artefact manifest, attestation manifest and platforms

	for _, image := range images.Items() {
		_, digestProvided := withDigests[image.Digest]
		outputInfo[image.Ref(true)] = imageInfo{
			Ref:                  image.Ref(true),
			DigestProvided:       digestProvided,
			Sources:              image.Sources,
			InlineAttestations:   map[string]document{},
			ExternalAttestations: map[string]document{},
			InlineSBOMs:          map[string]document{},
			ExternalSBOMs:        map[string]document{},
			InlineSignatures:     map[string]document{},
			ExternalSignatures:   map[string]document{},
			Related:              map[string][]oci.Metadata{},
		}
	}

	for _, image := range images.Items() {
		info := outputInfo[image.Ref(true)]

		index, err := client.Index(ctx, info.Ref)
		if err != nil {
			return fmt.Errorf("failed to get index for %s: %w", info.Ref, err)
		}

		//occurrances := len(image.Sources)
		//fmt.Printf("%s\t(digestProvided=%v occurrances=%d)\n", ref, !digestResolved, occurrances)

		related, err := client.ListRelated(ctx, image.OriginalName, image.Digest)
		if err != nil {
			return fmt.Errorf("failed to list related tag for %s: %w", info.Ref, err)
		}

		info.Related[image.Digest] = append(info.Related[image.Digest], related...)

		for _, manifest := range index.Manifests {
			info.Manifests = append(info.Manifests, imageManifest{
				Digest:      manifest.Digest,
				MediaType:   manifest.MediaType,
				Platform:    manifest.Platform,
				Annotations: manifest.Annotations,
			})
			// fmt.Printf("\t%s\t%s\t%s\n", manifest.Digest, manifest.MediaType, manifest.Platform.String())
			// for k, v := range manifest.Annotations {
			// 	fmt.Printf("\t\t%s=%s\n", k, v)
			// }

			digest := manifest.Digest.String()
			if v, ok := manifest.Annotations["vnd.docker.reference.type"]; ok && v == "attestation-manifest" {
				subject, ok := manifest.Annotations["vnd.docker.reference.digest"]
				if !ok {
					return fmt.Errorf("attestation manifest %s does not have 'vnd.docker.reference.digest' annotation", digest)
				}

				ref := image.OriginalName + "@" + digest
				artefact, err := fetchArtefact(ctx, client, ref)
				if err != nil {
					return fmt.Errorf("failed to fetch inline attestation: %w", err)
				}

				if artefact.MediaType != "application/vnd.in-toto+json" {
					return fmt.Errorf("unexpected media type of attestation in %q: %s", ref, artefact.MediaType)
				}

				doc := document{
					MediaType: string(artefact.MediaType),
					Object:    &in_toto.Statement{},
				}

				if err := json.NewDecoder(artefact).Decode(doc.Object); err != nil {
					return fmt.Errorf("failed to unmarshal attestation: %w", err)
				}

				info.InlineAttestations[subject] = doc
			}

			related, err := client.ListRelated(ctx, image.OriginalName, digest)
			if err != nil {
				return fmt.Errorf("failed to list related tag for %s: %w", info.Ref, err)
			}

			info.Related[digest] = append(info.Related[digest], related...)
		}

		for _, related := range info.Related {
			for _, metadata := range related {
				ref := metadata.URL + "@" + metadata.Digest
				switch {
				case strings.HasSuffix(metadata.URL, ".att"):
					artefact, err := fetchArtefact(ctx, client, ref)
					if err != nil {
						return fmt.Errorf("failed to fetch external attestation: %w", err)
					}

					if artefact.MediaType != "application/vnd.dsse.envelope.v1+json" {
						return fmt.Errorf("unexpected media type of attestation in %q: %s", ref, artefact.MediaType)
					}

					doc := document{
						MediaType: string(artefact.MediaType),
						Object:    &dsse.Envelope{},
					}

					if err := json.NewDecoder(artefact).Decode(doc.Object); err != nil {
						return fmt.Errorf("failed to unmarshal attestation: %w", err)
					}

					info.ExternalAttestations[ref] = doc
				case strings.HasSuffix(metadata.URL, ".sbom"):
					artefact, err := fetchArtefact(ctx, client, ref)
					if err != nil {
						return fmt.Errorf("failed to fetch external attestation: %w", err)
					}

					decoder := interface {
						Decode(any) error
					}(nil)

					switch artefact.MediaType {
					case "spdx+json":
						decoder = json.NewDecoder(artefact)
					default:
						return fmt.Errorf("unexpected media type of SBOM in %q: %s", ref, artefact.MediaType)
					}

					doc := document{
						MediaType: string(artefact.MediaType),
						Object:    any(nil),
					}

					if decoder.Decode(&doc.Object) != nil {
						return fmt.Errorf("failed to unmarshal SBOM: %w", err)
					}

					info.ExternalSBOMs[ref] = doc
				case strings.HasSuffix(metadata.URL, ".sig"):
					artefact, err := fetchArtefact(ctx, client, ref)
					if err != nil {
						return fmt.Errorf("failed to fetch external signature: %w", err)
					}

					if artefact.MediaType != "application/vnd.dev.cosign.simplesigning.v1+json" {
						return fmt.Errorf("unexpected media type of signature in %q: %s", ref, artefact.MediaType)
					}

					cosignBundleData, hasCosignBundle := artefact.Annotations["dev.sigstore.cosign/bundle"]
					if !hasCosignBundle {
						c.tape.log.Debugf("signature %q doesn't have bundle annotation", ref)
						c.tape.log.Debugf("signature %q annotations %#v", ref, artefact.Annotations)
						break
					}

					bundleObj := &struct {
						SignedEntryTimestamp string `json:"SignedEntryTimestamp"`
						Payload              struct {
							Body           string `json:"body"`
							IntegratedTime int    `json:"integratedTime"`
							LogIndex       int    `json:"logIndex"`
							LogID          string `json:"logID"`
						} `json:"Payload"`
					}{}

					if err := json.NewDecoder(strings.NewReader(cosignBundleData)).Decode(bundleObj); err != nil {
						return fmt.Errorf("failed to unmarshal signature bundle: %w", err)
					}

					hashedRekord := &struct {
						APIVersion string `json:"apiVersion"`
						Kind       string `json:"kind"`
						Spec       struct {
							Data struct {
								Hash struct {
									Algorithm string `json:"algorithm"`
									Value     string `json:"value"`
								} `json:"hash"`
							} `json:"data"`
							Signature struct {
								Content   string `json:"content"`
								PublicKey struct {
									Content string `json:"content"`
								} `json:"publicKey"`
							} `json:"signature"`
						} `json:"spec"`
					}{}

					if err := json.NewDecoder(newBase64Decoder(bundleObj.Payload.Body)).Decode(hashedRekord); err != nil {
						return fmt.Errorf("failed to unmarshal signature bundle: %w", err)
					}
					if hashedRekord.Kind != "hashedrekord" &&
						hashedRekord.APIVersion != "0.0.1" {
						return fmt.Errorf("unexpected signature bundle version and kind: %s/%s", hashedRekord.Kind, hashedRekord.APIVersion)
					}

					publicKeyPEM := bytes.NewBuffer(nil)
					_, err = io.Copy(publicKeyPEM, newBase64Decoder(hashedRekord.Spec.Signature.PublicKey.Content))
					if err != nil {
						return fmt.Errorf("failed to decode PEM signature bundle: %w", err)
					}
					certificates, err := cryptoutils.LoadCertificatesFromPEM(publicKeyPEM)
					if err != nil {
						return fmt.Errorf("failed to load certificates from PEM signature bundle: %w", err)
					}

					certificiatesInfo := struct {
						// TODO: add more info
						SubjectAlternativeNames [][]string
					}{}

					doc := document{
						MediaType: "application/vnd.com.docker.signinfo.v1alpha1", // TODO: define this as a constant
						Object:    certificiatesInfo,
					}

					for i := range certificates {
						certificiatesInfo.SubjectAlternativeNames = append(certificiatesInfo.SubjectAlternativeNames, cryptoutils.GetSubjectAlternateNames(certificates[i]))
					}

					info.ExternalSignatures[ref] = doc
				default:
					info.RelatedUnclassified = append(info.RelatedUnclassified, ref)
				}
			}
		}

		outputInfo[image.Ref(true)] = info
	}

	for _, info := range outputInfo {
		switch c.tape.OutputFormat {
		case OutputFormatDirectJSON:
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(info); err != nil {
				return fmt.Errorf("failed to marshal output: %w", err)
			}
		case OutputFormatText:
			// TODO: make this a method of the struct, perhaps use a tag for description
			fmt.Printf("%s\n", info.Ref)
			fmt.Printf("  Sources:\n")
			for _, source := range info.Sources {
				fmt.Printf("    %s %s:%d:%d@sha256:%s\n", source.OriginalRef, source.Manifest, source.Line, source.Column, source.ManifestDigest)
			}
			fmt.Printf("  Digest provided: %v\n", info.DigestProvided)

			if len(info.Manifests) > 0 {
				fmt.Printf("  Manifests:\n")
				for _, manifest := range info.Manifests {
					fmt.Printf("    %s  %s  %s\n", manifest.Digest, manifest.MediaType, manifest.Platform.String())
				}
			}

			fmt.Printf("  Related: %d\n", len(info.Related))
			fmt.Printf("  Inline attestations: %d\n", len(info.InlineAttestations))
			fmt.Printf("  External attestations: %d\n", len(info.ExternalAttestations))
			fmt.Printf("  Inline SBOMs: %d\n", len(info.InlineSBOMs))
			fmt.Printf("  External SBOMs: %d\n", len(info.ExternalSBOMs))
			fmt.Printf("  Inline signatures: %d\n", len(info.InlineSignatures))
			fmt.Printf("  External signatures: %d\n", len(info.ExternalSignatures))
			fmt.Printf("  Unclassified related tags: %d\n", len(info.RelatedUnclassified))
		default:
			return fmt.Errorf("unsupported output format: %s", c.tape.OutputFormat)
		}
	}
	return nil
}

func newBase64Decoder(data string) io.Reader {
	return base64.NewDecoder(base64.StdEncoding, strings.NewReader(data))
}

type AretfactInfo struct {
	io.ReadCloser

	MediaType   oci.MediaType
	Annotations map[string]string
}

func fetchArtefact(ctx context.Context, client *oci.Client, ref string) (*AretfactInfo, error) {
	image, err := client.Pull(ctx, ref)
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

	info := &AretfactInfo{
		ReadCloser:  blob,
		MediaType:   layerDecriptor.MediaType,
		Annotations: layerDecriptor.Annotations,
	}

	return info, nil
}
