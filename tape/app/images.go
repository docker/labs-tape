package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/in-toto/in-toto-golang/in_toto"

	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/sigstore/sigstore/pkg/cryptoutils"

	"github.com/errordeveloper/tape/manifest/imageresolver"
	"github.com/errordeveloper/tape/manifest/imagescanner"
	"github.com/errordeveloper/tape/manifest/loader"
	"github.com/errordeveloper/tape/manifest/types"
	"github.com/errordeveloper/tape/oci"
)

type TapeImagesCommand struct {
	tape *TapeCommand

	OutputFormatOptions
	InputManifestDirOptions
}

type imageManifest struct {
	Digest      oci.Hash          `json:"digest"`
	MediaType   oci.MediaType     `json:"mediaType"`
	Platform    *oci.Platform     `json:"platform,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Size        int64             `json:"size"`
}

type document struct {
	MediaType string `json:"mediaType"`
	Data      []byte `json:"data,omitempty"`
	Object    any    `json:"object,omitempty"`
}

type documents map[string]document

type imageInfo struct {
	Ref                  string                      `json:"ref"`
	Alias                *string                     `json:"alias,omitempty"`
	DigestProvided       bool                        `json:"digestProvided"`
	Sources              []types.Source              `json:"sources"`
	InlineAttestations   documents                   `json:"inlineAttestations"`
	ExternalAttestations documents                   `json:"externalAttestations"`
	InlineSBOMs          documents                   `json:"inlineSBOMs,omitempty"`
	ExternalSBOMs        documents                   `json:"externalSBOMs,omitempty"`
	InlineSignatures     documents                   `json:"inlineSignatures,omitempty"` // TODO: implement
	ExternalSignatures   documents                   `json:"externalSignatures,omitempty"`
	Related              map[string]*types.ImageList `json:"related,omitempty"`
	RelatedUnclassified  []string                    `json:"relatedUnclassified,omitempty"`
	Manifests            []imageManifest             `json:"manifests,omitempty"`
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

	client := oci.NewClient(nil) // oci.NewDebugClient(os.Stdout, nil)
	// TODO: use client.LoginWithCredentials() and/or other options
	// TODO: integrate with docker-credential-helpers

	resolver := imageresolver.NewRegistryResolver(client)

	outputInfo, err := c.CollectInfo(ctx, images, client, resolver)
	if err != nil {
		return fmt.Errorf("failed to collect info about images: %w", err)
	}

	if err := c.PrintInfo(ctx, outputInfo); err != nil {
		return fmt.Errorf("failed to print info about images: %w", err)
	}

	return nil
}

func (c *TapeImagesCommand) CollectInfo(ctx context.Context, images *types.ImageList, client *oci.Client, resolver imageresolver.Resolver) (map[string]imageInfo, error) {
	outputInfo := make(map[string]imageInfo, len(images.Items()))

	withDigests := map[string]struct{}{}
	for _, image := range images.Items() {
		if image.Digest != "" {
			withDigests[image.Digest] = struct{}{}
		}
	}

	c.tape.log.Info("resolving image digests")
	if err := resolver.ResolveDigests(ctx, images); err != nil {
		return nil, fmt.Errorf("failed to resolve image digests: %w", err)
	}

	if err := images.Dedup(); err != nil {
		return nil, fmt.Errorf("failed to dedup images: %w", err)
	}

	// TODO: improve JSON formatter
	// TODO: attestation formatter
	// TODO: add test for the the CLI functionality, as fetching related tags is not covered by package tests
	// TODO: include info about signatures
	// TODO: include references to manifests where image is used

	for _, image := range images.Items() {
		_, digestProvided := withDigests[image.Digest]
		outputInfo[image.Ref(true)] = imageInfo{
			Ref:                  image.Ref(true),
			Alias:                image.Alias,
			DigestProvided:       digestProvided,
			Sources:              image.Sources,
			InlineAttestations:   map[string]document{},
			ExternalAttestations: map[string]document{},
			InlineSBOMs:          map[string]document{},
			ExternalSBOMs:        map[string]document{},
			InlineSignatures:     map[string]document{},
			ExternalSignatures:   map[string]document{},
			Related:              map[string]*types.ImageList{},
		}
	}

	c.tape.log.Info("resolving related images")

	related, err := resolver.FindRelatedTags(ctx, images)
	if err != nil {
		return nil, fmt.Errorf("failed to find related tags: %w", err)
	}

	c.tape.log.Debugf("related images: %#v", related.Items())

	inspectManifest := func(image *types.Image, imageIndex oci.ImageIndex, indexManifest *oci.IndexManifest) error {
		info := outputInfo[image.Ref(true)]

		for _, manifest := range indexManifest.Manifests {
			info.Manifests = append(info.Manifests, imageManifest{
				Digest:      manifest.Digest,
				MediaType:   manifest.MediaType,
				Platform:    manifest.Platform,
				Size:        manifest.Size,
				Annotations: manifest.Annotations,
			})
		}

		artefacts, _, err := client.FetchFromIndexOrImage(ctx, imageIndex, indexManifest, nil, "application/vnd.in-toto+json")
		if err != nil {
			return fmt.Errorf("failed to fetch inline attestation: %w", err)
		}

		for _, artefact := range artefacts {

			doc := document{
				MediaType: string(artefact.MediaType),
				Object:    &in_toto.Statement{},
			}

			if err := json.NewDecoder(artefact).Decode(doc.Object); err != nil {
				return fmt.Errorf("failed to unmarshal attestation: %w", err)
			}

			var subject string
			if v, ok := artefact.Annotations["vnd.docker.reference.type"]; ok && v == "attestation-manifest" {
				subject, ok = artefact.Annotations["vnd.docker.reference.digest"]
				if !ok {
					return fmt.Errorf("attestation manifest %q does not have 'vnd.docker.reference.digest' annotation", artefact.Digest)
				}
			} else {
				statementSubject := doc.Object.(*in_toto.Statement).Subject
				if len(statementSubject) == 0 {
					return fmt.Errorf("statement in %q does not have a subject", artefact.Digest)
				}
				subject, ok = statementSubject[0].Digest["sha256"]
				if !ok {
					return fmt.Errorf("first subject in %q does not have a sha256 digest", artefact.Digest)
				}
			}
			if subject == "" {
				return fmt.Errorf("invalid inline attestation in %q: unable to determine subject", artefact.Digest)
			}

			if predicateType, ok := artefact.Annotations["in-toto.io/predicate-type"]; ok {
				switch predicateType {
				case "https://spdx.dev/Document":
					if _, ok := info.InlineSBOMs[subject]; ok {
						return fmt.Errorf("duplicate SBOM for %s", subject)
					}
					info.InlineSBOMs[subject] = doc
				default:
					if _, ok := info.InlineAttestations[subject]; ok {
						return fmt.Errorf("duplicate inline attestation for %s", subject)
					}
					info.InlineAttestations[subject] = doc
				}
			}
		}

		outputInfo[image.Ref(true)] = info
		return nil
	}

	manifests, relatedToManifests, err := resolver.FindRelatedFromIndecies(ctx, images, inspectManifest)
	if err != nil {
		return nil, err
	}

	for _, image := range images.Items() {
		imageRef := image.Ref(true)
		info := outputInfo[imageRef]

		if relatedImages := related.CollectRelatedToRef(imageRef); relatedImages.Len() > 0 {
			info.Related[imageRef] = relatedImages
		}

		for _, manifestRef := range manifests.RelatedTo(imageRef) {
			if relatedImages := relatedToManifests.CollectRelatedToRef(manifestRef); relatedImages.Len() > 0 {
				info.Related[manifestRef] = relatedImages
			}
		}

		for _, related := range info.Related {
			for _, relatedImage := range related.Items() {
				ref := relatedImage.Ref(true)
				switch {
				case strings.HasSuffix(relatedImage.OriginalTag, ".att"):
					artefact, err := client.GetSingleArtefact(ctx, ref)
					if err != nil {
						return nil, fmt.Errorf("failed to fetch external attestation: %w", err)
					}

					if artefact.MediaType != "application/vnd.dsse.envelope.v1+json" {
						return nil, fmt.Errorf("unexpected media type of attestation in %q: %s", ref, artefact.MediaType)
					}

					doc := document{
						MediaType: string(artefact.MediaType),
						Object:    &dsse.Envelope{},
					}

					if err := json.NewDecoder(artefact).Decode(doc.Object); err != nil {
						return nil, fmt.Errorf("failed to unmarshal attestation: %w", err)
					}

					info.ExternalAttestations[ref] = doc
				case strings.HasSuffix(relatedImage.OriginalTag, ".sbom"):
					artefact, err := client.GetSingleArtefact(ctx, ref)
					if err != nil {
						return nil, fmt.Errorf("failed to fetch external attestation: %w", err)
					}

					decoder := interface {
						Decode(any) error
					}(nil)

					switch artefact.MediaType {
					case "spdx+json":
						decoder = json.NewDecoder(artefact)
					default:
						return nil, fmt.Errorf("unexpected media type of SBOM in %q: %s", ref, artefact.MediaType)
					}

					doc := document{
						MediaType: string(artefact.MediaType),
						Object:    any(nil),
					}

					if decoder.Decode(&doc.Object) != nil {
						return nil, fmt.Errorf("failed to unmarshal SBOM: %w", err)
					}

					info.ExternalSBOMs[ref] = doc
				case strings.HasSuffix(relatedImage.OriginalTag, ".sig"):
					artefact, err := client.GetSingleArtefact(ctx, ref)
					if err != nil {
						return nil, fmt.Errorf("failed to fetch external signature: %w", err)
					}

					if artefact.MediaType != "application/vnd.dev.cosign.simplesigning.v1+json" {
						return nil, fmt.Errorf("unexpected media type of signature in %q: %s", ref, artefact.MediaType)
					}

					cosignBundleData, hasCosignBundle := artefact.Annotations["dev.sigstore.cosign/bundle"]
					if !hasCosignBundle {
						c.tape.log.Debugf("signature %q doesn't have bundle annotation", ref)
						c.tape.log.Debugf("signature %q annotations %#v", ref, artefact.Annotations)
						info.ExternalSignatures[ref] = document{
							MediaType: "application/vnd.com.docker.signinfo.v1alpha1", // TODO: define this as a constant
							Object:    nil,
						}
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
						return nil, fmt.Errorf("failed to unmarshal signature bundle: %w", err)
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
						return nil, fmt.Errorf("failed to unmarshal signature bundle: %w", err)
					}
					if hashedRekord.Kind != "hashedrekord" &&
						hashedRekord.APIVersion != "0.0.1" {
						return nil, fmt.Errorf("unexpected signature bundle version and kind: %s/%s", hashedRekord.Kind, hashedRekord.APIVersion)
					}

					publicKeyPEM := bytes.NewBuffer(nil)
					_, err = io.Copy(publicKeyPEM, newBase64Decoder(hashedRekord.Spec.Signature.PublicKey.Content))
					if err != nil {
						return nil, fmt.Errorf("failed to decode PEM signature bundle: %w", err)
					}
					certificates, err := cryptoutils.LoadCertificatesFromPEM(publicKeyPEM)
					if err != nil {
						return nil, fmt.Errorf("failed to load certificates from PEM signature bundle: %w", err)
					}

					certificiatesInfo := struct {
						// TODO: add more info
						SubjectAlternativeNames [][]string
					}{}

					for i := range certificates {
						certificiatesInfo.SubjectAlternativeNames = append(certificiatesInfo.SubjectAlternativeNames, cryptoutils.GetSubjectAlternateNames(certificates[i]))
					}

					info.ExternalSignatures[ref] = document{
						MediaType: "application/vnd.com.docker.signinfo.v1alpha1", // TODO: define this as a constant
						Object:    certificiatesInfo,
					}
				default:
					info.RelatedUnclassified = append(info.RelatedUnclassified, ref)
				}
			}
		}

		outputInfo[imageRef] = info
	}
	return outputInfo, nil
}

func (c *TapeImagesCommand) PrintInfo(ctx context.Context, outputInfo map[string]imageInfo) error {
	stdj := json.NewEncoder(os.Stdout)

	for _, info := range outputInfo {
		switch c.OutputFormat {
		case OutputFormatDirectJSON:
			stdj.SetIndent("", "  ")
			if err := stdj.Encode(info); err != nil {
				return fmt.Errorf("failed to marshal output: %w", err)
			}
		case OutputFormatText, OutputFormatDetailedText:
			// TODO: make this a method of the struct, perhaps use a tag for description
			fmt.Printf("%s\n", info.Ref)
			if info.Alias != nil {
				fmt.Printf("  Alias: %s\n", *info.Alias)

			}
			fmt.Printf("  Sources:\n")
			for _, source := range info.Sources {
				fmt.Printf("    %s %s:%d:%d@sha256:%s\n", source.OriginalRef, source.Manifest, source.Line, source.Column, source.ManifestDigest)
			}
			fmt.Printf("  Digest provided: %v\n", info.DigestProvided)

			if len(info.Manifests) > 0 {
				fmt.Printf("  OCI manifests:\n")
				for _, manifest := range info.Manifests {
					fmt.Printf("    %s  %s  %s  %d\n", manifest.Digest, manifest.MediaType, manifest.Platform.String(), manifest.Size)
				}
			}

			docsets := map[string]map[string]document{
				"Inline attestations":   info.InlineAttestations,
				"External attestations": info.ExternalAttestations,
				"Inline SBOMs":          info.InlineSBOMs,
				"External SBOMs":        info.ExternalSBOMs,
				"Inline signatures":     info.InlineSignatures,
				"External signatures":   info.ExternalSignatures,
			}

			if c.OutputFormat == OutputFormatText {
				for desc, docset := range docsets {
					fmt.Printf("  %s: %d\n", desc, len(docset))
				}
				break
			}

			if len(info.Related) > 0 {
				fmt.Printf("  Related tags:\n")
				for relatedTo, related := range info.Related {
					for _, relatedImage := range related.Items() {
						fmt.Printf("   %s  %s\n", relatedTo, relatedImage.Ref(true))
					}
				}
			}
			if len(info.RelatedUnclassified) > 0 {
				fmt.Printf("  Related unclassified:\n")
				for _, ref := range info.RelatedUnclassified {
					fmt.Printf("    %s\n", ref)
				}
			}
			for desc, docset := range docsets {
				if len(docset) == 0 {
					fmt.Printf("  %s: <none>\n", desc)
				} else {
					fmt.Printf("  %s:\n", desc)
				}
				for subject, doc := range docset {
					if doc.Object == nil && len(doc.Data) == 0 {
						fmt.Printf("    %s %s: <none>\n", subject, doc.MediaType)
					}
					if doc.Object != nil {
						fmt.Printf("    %s %s:\n        ", subject, doc.MediaType)
						stdj.SetIndent("        ", "  ")
						if err := stdj.Encode(doc.Object); err != nil {
							return fmt.Errorf("failed to marshal output: %w", err)
						}
					}
					if len(doc.Data) > 0 {
						fmt.Printf("    %s %s: %s\n", subject, doc.MediaType, base64.RawStdEncoding.EncodeToString(doc.Data))
					}
				}
			}
		default:
			return fmt.Errorf("unsupported output format: %s", c.OutputFormat)
		}
	}
	return nil
}

func newBase64Decoder(data string) io.Reader {
	return base64.NewDecoder(base64.StdEncoding, strings.NewReader(data))
}
