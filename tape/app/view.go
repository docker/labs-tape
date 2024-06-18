package app

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"

	toto "github.com/in-toto/in-toto-golang/in_toto"

	"github.com/errordeveloper/tape/attest/manifest"
	attestTypes "github.com/errordeveloper/tape/attest/types"

	"github.com/errordeveloper/tape/oci"
)

type TapeViewCommand struct {
	tape *TapeCommand
	OutputFormatOptions

	Image string `short:"I" long:"image" description:"Name of the image to view" required:"true"`
}

type artefactInfo struct {
	AppImages    []string `json:"appImages"`
	RawManifests struct {
		Index   rawManifest[oci.IndexManifest] `json:"index"`
		Content rawManifest[oci.Manifest]      `json:"content"`
		Attest  rawManifest[oci.Manifest]      `json:"attest"`
	} `json:"rawManifests"`
	Attestations        []toto.Statement               `json:"attestations"`
	AttestationsSummary *attestTypes.SummaryAnnotation `json:"attestationsSummary,omitempty"`
}

type rawManifest[T oci.Manifest | oci.IndexManifest] struct {
	Digest   string `json:"digest,omitempty"`
	Manifest *T     `json:"manifest,omitempty"`
}

func (c *TapeViewCommand) Execute(args []string) error {
	ctx := context.WithValue(c.tape.ctx, "command", "view")
	if len(args) != 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}

	if err := c.tape.Init(); err != nil {
		return err
	}

	client := oci.NewClient(nil)

	outputInfo, err := c.CollectInfo(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to collect info about artifact: %w", err)
	}

	if err := c.PrintInfo(ctx, outputInfo); err != nil {
		return fmt.Errorf("failed to print info about artifact: %w", err)
	}

	return nil
}

func (c *TapeViewCommand) CollectInfo(ctx context.Context, client *oci.Client) (*artefactInfo, error) {
	artefactInfo := &artefactInfo{}

	imageIndex, indexManifest, _, err := client.GetIndexOrImage(ctx, c.Image)
	if err != nil {
		return nil, err
	}
	if indexManifest == nil {
		return nil, fmt.Errorf("no index manifest found for %q", c.Image)
	}

	imageIndexDigest, err := imageIndex.Digest()
	if err != nil {
		return nil, err
	}

	artefactInfo.RawManifests.Index = rawManifest[oci.IndexManifest]{
		Digest:   imageIndexDigest.String(),
		Manifest: indexManifest,
	}

	imageInfo, manifests, err := client.FetchFromIndexOrImage(ctx, imageIndex, indexManifest, nil)
	if err != nil {
		return nil, err
	}

	if len(imageInfo) == 0 {
		return nil, fmt.Errorf("no images found in index %q", c.Image)
	}

	for i := range imageInfo {
		info := imageInfo[i]
		switch info.MediaType {
		case oci.ContentMediaType:
		case oci.AttestMediaType:
			if annotation, ok := info.Annotations[oci.AttestationsSummaryAnnotation]; ok {
				summary, err := attestTypes.UnmarshalSummaryAnnotation(annotation)
				if err != nil {
					return nil, err
				}
				artefactInfo.AttestationsSummary = summary
			}

			gr, err := gzip.NewReader(info)
			if err != nil {
				return nil, err
			}
			scanner := bufio.NewScanner(gr)
			for scanner.Scan() {
				statement := toto.Statement{} // attestTypes.GenericStatement[any]{}
				if err := json.NewDecoder(bytes.NewBuffer(scanner.Bytes())).Decode(&statement); err != nil {
					return nil, err
				}
				artefactInfo.Attestations = append(artefactInfo.Attestations, statement)
			}
			if err := scanner.Err(); err != nil {
				return nil, err
			}
			if err := gr.Close(); err != nil {
				return nil, err
			}
		}
	}

	for _, statement := range artefactInfo.Attestations {
		if statement.PredicateType == manifest.ReplacedImageRefPredicateType {
			buf := bytes.NewBuffer(nil)
			if err := json.NewEncoder(buf).Encode(statement.Predicate); err != nil {
				return nil, err
			}
			predicate := &struct {
				manifest.ImageRefenceWithLocation `json:"replacedImageReference"`
			}{}
			if err := json.NewDecoder(buf).Decode(predicate); err != nil {
				return nil, err
			}

			artefactInfo.AppImages = append(artefactInfo.AppImages, predicate.Reference)
		}
	}

	for digest := range manifests {
		m := rawManifest[oci.Manifest]{
			Digest:   digest.String(),
			Manifest: manifests[digest],
		}
		switch m.Manifest.Config.MediaType {
		case oci.ContentMediaType:
			artefactInfo.RawManifests.Content = m
		case oci.AttestMediaType:
			artefactInfo.RawManifests.Attest = m
		}
	}
	return artefactInfo, nil
}

func (c *TapeViewCommand) PrintInfo(ctx context.Context, outputInfo *artefactInfo) error {
	stdj := json.NewEncoder(os.Stdout)
	switch c.OutputFormat {
	case OutputFormatDirectJSON:
		stdj.SetIndent("", "  ")
		if err := stdj.Encode(outputInfo); err != nil {
			return fmt.Errorf("failed to marshal output: %w", err)
		}
	case OutputFormatText, OutputFormatDetailedText:
		fmt.Printf("%s\n", c.Image)
		fmt.Printf("  Digest: %s\n", outputInfo.RawManifests.Index.Digest)
		fmt.Printf("  OCI Manifests:\n")
		fmt.Printf("    %s %s\n", outputInfo.RawManifests.Content.Digest,
			outputInfo.RawManifests.Content.Manifest.Config.MediaType)
		fmt.Printf("    %s %s\n", outputInfo.RawManifests.Attest.Digest,
			outputInfo.RawManifests.Attest.Manifest.Config.MediaType)
		if len(outputInfo.AppImages) > 0 {
			fmt.Printf("  App Images:\n")
			for i := range outputInfo.AppImages {
				fmt.Printf("    %s\n", outputInfo.AppImages[i])
			}
		}
		if outputInfo.AttestationsSummary != nil {
			fmt.Printf("  Attestations Summary:\n")
			fmt.Printf("    Number of Statements: %v\n", outputInfo.AttestationsSummary.NumStamentes)
			if outputInfo.AttestationsSummary.NumStamentes > 0 {
				fmt.Printf("      Predicate Types:\n")
				for i := range outputInfo.AttestationsSummary.PredicateTypes {
					fmt.Printf("        %s\n", outputInfo.AttestationsSummary.PredicateTypes[i])
				}
				fmt.Printf("      Subjects:\n")
				for i := range outputInfo.AttestationsSummary.Subjects {
					fmt.Printf("        %s@sha256:%s\n", outputInfo.AttestationsSummary.Subjects[i].Name, outputInfo.AttestationsSummary.Subjects[i].Digest.String())
				}
			}
		}
	}
	return nil
}
