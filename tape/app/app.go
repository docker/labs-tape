package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	flags "github.com/thought-machine/go-flags"

	"github.com/docker/staples/pkg/logger"

	ociclient "github.com/fluxcd/pkg/oci/client"

	"github.com/docker/labs-brown-tape/manifest/imageresolver"
	"github.com/docker/labs-brown-tape/manifest/imagescanner"
	"github.com/docker/labs-brown-tape/manifest/loader"
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
			options: &TapeImagesCommand{tape: tape},
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

	client := ociclient.NewClient(nil)
	// TODO: use client.LoginWithCredentials() and/or other options
	// TODO: integrate with docker-credential-helpers

	if err := imageresolver.NewRegistryResolver(client).ResolveDigests(c.tape.ctx, images); err != nil {
		return fmt.Errorf("failed to resolve image digests: %w", err)
	}

	for _, image := range images.Items() {
		fmt.Println(image.Ref(true))
	}

	// TODO: json formatter
	// TODO: attempt call `cosign download signature` and find out who signed the image
	// TODO: get OCI annotations, artefact manifest, attestation manifest and platforms
	return nil
}
