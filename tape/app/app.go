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

	"github.com/google/go-containerregistry/pkg/v1/types"
	flags "github.com/thought-machine/go-flags"

	"github.com/docker/staples/pkg/logger"

	"github.com/sigstore/sigstore/pkg/cryptoutils"

	"github.com/docker/labs-brown-tape/manifest/imageresolver"
	"github.com/docker/labs-brown-tape/manifest/imagescanner"
	"github.com/docker/labs-brown-tape/manifest/loader"
	"github.com/docker/labs-brown-tape/oci"
)

type TapeCommand struct {
	LogLevel string `long:"log-level" description:"Log level" default:"debug"`

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

	c.tape.log.Info("resolving image digests")

	client := oci.NewClient(nil)
	// TODO: use client.LoginWithCredentials() and/or other options
	// TODO: integrate with docker-credential-helpers

	digestProvided := map[string]struct{}{}
	for _, image := range images.Items() {
		if image.Digest != "" {
			digestProvided[image.Ref(true)] = struct{}{}
		}
	}

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

	images.Dedup()

	for _, image := range images.Items() {
		ref := image.Ref(true)

		index, err := client.Index(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to get index for %s: %w", ref, err)
		}
		digestResolved := true
		if _, ok := digestProvided[ref]; ok {
			digestResolved = false
		}
		fmt.Printf("%s\t(digestProvided=%v occurrances=%d)\n", ref, !digestResolved, len(image.Sources))

		for _, source := range image.Sources {
			fmt.Printf("\t%s:%d:%d (%s)\n", source.Manifest, source.Line, source.Column, source.ManifestDigest)
		}

		relatedTo := map[string][]oci.Metadata{}

		related, err := client.ListRelated(ctx, image.OriginalName, image.Digest)
		if err != nil {
			return fmt.Errorf("failed to list related tag for %s: %w", ref, err)
		}

		if len(related) > 0 {
			relatedTo[image.Digest] = append(relatedTo[image.Digest], related...)
		}

		inlineAttestations := map[string]string{}
		for _, manifest := range index.Manifests {
			fmt.Printf("\t%s\t%s\t%s\n", manifest.Digest, manifest.MediaType, manifest.Platform.String())
			for k, v := range manifest.Annotations {
				fmt.Printf("\t\t%s=%s\n", k, v)
			}
			digest := manifest.Digest.String()
			if v, ok := manifest.Annotations["vnd.docker.reference.type"]; ok && v == "attestation-manifest" {
				subject, ok := manifest.Annotations["vnd.docker.reference.digest"]
				if !ok {
					return fmt.Errorf("attestation manifest %s does not have 'vnd.docker.reference.digest' annotation", digest)
				}
				inlineAttestations[subject] = digest
			}

			related, err := client.ListRelated(ctx, image.OriginalName, digest)
			if err != nil {
				return fmt.Errorf("failed to list related tag for %s: %w", ref, err)
			}

			if len(related) > 0 {
				relatedTo[digest] = append(relatedTo[digest], related...)
			}
		}

		externalAttestations := map[string]string{}
		externalSBOMs := map[string]string{}
		externalSignatures := map[string]string{}

		if len(relatedTo) > 0 {
			for _, related := range relatedTo {
				for _, metadata := range related {
					ref := metadata.URL + "@" + metadata.Digest
					switch {
					case strings.HasSuffix(metadata.URL, ".att"):
						externalAttestations[ref] = ""

					case strings.HasSuffix(metadata.URL, ".sbom"):
						externalSBOMs[ref] = ""
					case strings.HasSuffix(metadata.URL, ".sig"):
						externalSignatures[ref] = ""
					default:
						fmt.Printf("\t\tunrecognised related tag: %s\n", ref)
					}
				}
			}
		}

		if len(inlineAttestations) > 0 {
			fmt.Printf("\tinline attestations:\n")
		}
		for subject, attestation := range inlineAttestations {
			ref := image.OriginalName + "@" + attestation
			artefact, err := fetchArtefact(ctx, client, ref)
			if err != nil {
				return fmt.Errorf("failed to fetch inline attestation: %w", err)
			}

			if artefact.MediaType != "application/vnd.in-toto+json" {
				return fmt.Errorf("unexpected media type of attestation in %q: %s", ref, artefact.MediaType)
			}

			data, _ := io.ReadAll(artefact)

			inlineAttestations[subject] = string(data)
			fmt.Printf("\t\t%s\n", string(data))
		}

		if len(externalAttestations) > 0 {
			fmt.Printf("\texternal attestations:\n")
		}
		for ref := range externalAttestations {
			artefact, err := fetchArtefact(ctx, client, ref)
			if err != nil {
				return fmt.Errorf("failed to fetch external attestation: %w", err)
			}

			if artefact.MediaType != "application/vnd.dsse.envelope.v1+json" {
				return fmt.Errorf("unexpected media type of attestation in %q: %s", ref, artefact.MediaType)
			}

			data, _ := io.ReadAll(artefact)

			externalAttestations[ref] = string(data)
			fmt.Printf("\t\t%s\t%s\n", artefact.MediaType, string(data))
		}

		if len(externalSBOMs) > 0 {
			fmt.Printf("\texternal SBOMs:\n")
			// print refs with digests for now
			for ref := range externalSBOMs {
				fmt.Printf("\t\t%s\n", ref)
			}
		}
		if len(externalSignatures) > 0 {
			fmt.Printf("\texternal signatures:\n")
			for ref := range externalSignatures {
				artefact, err := fetchArtefact(ctx, client, ref)
				if err != nil {
					return fmt.Errorf("failed to fetch external signature: %w", err)
				}

				if artefact.MediaType != "application/vnd.dev.cosign.simplesigning.v1+json" {
					return fmt.Errorf("unexpected media type of signature in %q: %s", ref, artefact.MediaType)
				}

				data, _ := io.ReadAll(artefact)

				fmt.Printf("\t\t%s\t%s\n", artefact.MediaType, string(data))
				// for k, v := range artefact.Annotations {
				// 	fmt.Printf("\t\t\t%s=%s\n", k, base64.StdEncoding.EncodeToString([]byte(v)))
				// }
				cosignBundleData, hasCosignBundle := artefact.Annotations["dev.sigstore.cosign/bundle"]
				if !hasCosignBundle {
					c.tape.log.Debugf("signature %q annotations %#v", ref, artefact.Annotations)
					return fmt.Errorf("signature %q does not have 'dev.sigstore.cosign/bundle' annotation", ref)
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

				hashedrekord := &struct {
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

				if err := json.NewDecoder(newBase64Decoder(bundleObj.Payload.Body)).Decode(hashedrekord); err != nil {
					return fmt.Errorf("failed to unmarshal signature bundle: %w", err)
				}
				if hashedrekord.Kind != "hashedrekord" &&
					hashedrekord.APIVersion != "0.0.1" {
					return fmt.Errorf("unexpected signature bundle version and kind: %s/%s", hashedrekord.Kind, hashedrekord.APIVersion)
				}

				publicKeyPEM := bytes.NewBuffer(nil)
				_, err = io.Copy(publicKeyPEM, newBase64Decoder(hashedrekord.Spec.Signature.PublicKey.Content))
				if err != nil {
					return fmt.Errorf("failed to decode PEM signature bundle: %w", err)
				}
				certificates, err := cryptoutils.LoadCertificatesFromPEM(publicKeyPEM)
				if err != nil {
					return fmt.Errorf("failed to load certificates from PEM signature bundle: %w", err)
				}

				for i := range certificates {
					fmt.Printf("%#v\n", cryptoutils.GetSubjectAlternateNames(certificates[i]))
				}
			}
		}
	}

	return nil
}

func newBase64Decoder(data string) io.Reader {
	return base64.NewDecoder(base64.StdEncoding, strings.NewReader(data))
}

type AretfactInfo struct {
	io.ReadCloser

	MediaType   types.MediaType
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
