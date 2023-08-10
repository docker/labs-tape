package app

import (
	"context"
	"os"
	"os/signal"
	"strings"

	flags "github.com/thought-machine/go-flags"

	"github.com/docker/labs-brown-tape/logger"
)

type OutputFormat string

const (
	OutputFormatDetailedText OutputFormat = "detailed-text"
	OutputFormatText         OutputFormat = "text"
	OutputFormatDirectJSON   OutputFormat = "direct-json"
)

type TapeCommand struct {
	LogLevel     string       `short:"l" long:"log-level" description:"Log level" default:"info"`
	OutputFormat OutputFormat `short:"o" long:"output-format" description:"Format of the output to use" default:"detailed-text"`

	log *logger.Logger
	ctx context.Context
}

type CommonOptions struct {
	ManifestDir string `short:"D" long:"manifest-dir" description:"Directory containing manifests" required:"true"`

	tape *TapeCommand
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
		options flags.Commander
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
		{
			name:  "package",
			short: "Create a package",
			long: []string{
				"This command can be used to package app images and configuration",
				"as a single self-contained artefact",
			},
			options: &TapePackageCommand{
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
