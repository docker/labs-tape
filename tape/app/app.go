package app

import (
	"context"
	"os"
	"os/signal"
	"strings"

	flags "github.com/thought-machine/go-flags"

	"github.com/errordeveloper/tape/logger"
)

type OutputFormat string

const (
	OutputFormatDetailedText OutputFormat = "detailed-text"
	OutputFormatText         OutputFormat = "text"
	OutputFormatDirectJSON   OutputFormat = "direct-json"
)

type TapeCommand struct {
	LogLevel string `short:"l" long:"log-level" description:"Log level" default:"info"`

	log *logger.Logger
	ctx context.Context
}

type OutputFormatOptions struct {
	OutputFormat OutputFormat `short:"o" long:"output-format" description:"Format of the output to use" default:"detailed-text"`
}

type InputManifestDirOptions struct {
	ManifestDir string `short:"D" long:"manifest-dir" description:"Intput directory to read manifests from" required:"true"`
}

type OutputManifestDirOptions struct {
	ManifestDir string `short:"D" long:"manifest-dir" description:"Output directory to exact manifests" required:"true"`
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
				"This command load manifests from the given dir and prints info about",
				"all app images referenced in these manifests",
			},
			options: &TapeImagesCommand{
				tape:                    tape,
				InputManifestDirOptions: InputManifestDirOptions{},
			},
		},
		{
			name:  "package",
			short: "Package an artefact",
			long: []string{
				"This command process manifests from the given dir and packages them as an artefact",
			},
			options: &TapePackageCommand{
				tape:                    tape,
				InputManifestDirOptions: InputManifestDirOptions{}},
		},
		{
			name:  "pull",
			short: "Pull an artefact",
			options: &TapePullCommand{
				tape:                     tape,
				OutputManifestDirOptions: OutputManifestDirOptions{},
			},
		},
		{
			name:  "view",
			short: "View an artefact",
			options: &TapeViewCommand{
				tape: tape,
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
