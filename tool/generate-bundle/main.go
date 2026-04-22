package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/desertwitch/par2cron/internal/bundle"
	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/spf13/afero"
)

const programName = "tool/generate-bundle"

var manifest = bundle.ManifestInput{
	Name:  "manifest.json",
	Bytes: []byte(`{"version":1,"description":"reference bundle"}`),
}

type Options struct {
	Dir   string
	Out   string
	Parse string
	Files []string
}

func (o Options) Validate() error {
	if o.Parse == "" {
		return errors.New("-parse flag is required")
	}
	if len(o.Files) == 0 {
		return errors.New("at least one input file must be given")
	}

	return nil
}

type Service struct {
	fsys    afero.Fs
	par2er  schema.Par2Handler
	bundler schema.BundleHandler
}

type bundleHandler struct{}

func (bundleHandler) Open(fsys afero.Fs, bundlePath string) (schema.Bundle, error) { //nolint:ireturn
	return bundle.Open(fsys, bundlePath) //nolint:wrapcheck
}

func (bundleHandler) Pack(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
	return bundle.Pack(fsys, bundlePath, recoverySetID, manifest, files) //nolint:wrapcheck
}

type par2Handler struct{}

func (par2Handler) ParseFile(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
	return par2.ParseFile(fsys, path, panicAsErr) //nolint:wrapcheck
}

func NewService(fsys afero.Fs, par2er schema.Par2Handler, bundler schema.BundleHandler) *Service {
	return &Service{
		fsys:    fsys,
		par2er:  par2er,
		bundler: bundler,
	}
}

func (s *Service) Run(opts Options) (string, error) {
	if err := opts.Validate(); err != nil {
		return "", fmt.Errorf("args error: %w", err)
	}

	parsePath := filepath.Join(opts.Dir, opts.Parse)
	pf, err := s.par2er.ParseFile(s.fsys, parsePath, true)
	if err != nil {
		return "", fmt.Errorf("parse error: %w", err)
	}

	if len(pf.Sets) < 1 || pf.Sets[0].MainPacket == nil {
		return "", errors.New("parsed file has no sets or main packet")
	}

	recoverySetID := pf.Sets[0].MainPacket.SetID

	inputs := make([]bundle.FileInput, len(opts.Files))
	for i, name := range opts.Files {
		inputs[i] = bundle.FileInput{
			Name: filepath.Base(name),
			Path: filepath.Join(opts.Dir, name),
		}
	}

	outPath := filepath.Join(opts.Dir, opts.Out)
	if err := s.fsys.MkdirAll(filepath.Dir(outPath), 0o755); err != nil { //nolint:mnd
		return "", fmt.Errorf("fs error: %w", err)
	}

	if err := s.bundler.Pack(s.fsys, outPath, recoverySetID, manifest, inputs); err != nil {
		return "", fmt.Errorf("pack error: %w", err)
	}

	bun, err := s.bundler.Open(s.fsys, outPath)
	if err != nil {
		return "", fmt.Errorf("bundle open error: %w", err)
	}
	defer bun.Close()

	if err := bun.Validate(true); err != nil {
		return "", fmt.Errorf("bundle validate error: %w", err)
	}

	return outPath, nil
}

func parseArgs(args []string) (Options, error) {
	flags := flag.NewFlagSet(programName, flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	dir := flags.String("dir", "testdata", "base directory for all file paths")
	out := flags.String("out", "output.bun.par2", "output filename relative to -dir")
	parse := flags.String("parse", "", "index .par2 file relative to -dir")

	if err := flags.Parse(args); err != nil {
		return Options{}, fmt.Errorf("args error: %w", err)
	}

	opts := Options{
		Dir:   *dir,
		Out:   *out,
		Parse: *parse,
		Files: flags.Args(),
	}
	if err := opts.Validate(); err != nil {
		return Options{}, fmt.Errorf("args error: %w", err)
	}

	return opts, nil
}

func run(args []string, stdout io.Writer, service *Service) error {
	opts, err := parseArgs(args)
	if err != nil {
		return err
	}

	outPath, err := service.Run(opts)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(stdout, "%s: success: %s\n", programName, outPath); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	return nil
}

func main() {
	service := NewService(afero.NewOsFs(), par2Handler{}, bundleHandler{})
	if err := run(os.Args[1:], os.Stdout, service); err != nil {
		log.Fatalf("%s: %v", programName, err)
	}
}
