//nolint:gosec
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/desertwitch/par2cron/internal/bundle"
	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/spf13/afero"
)

const programName = "tool/generate-bundle"

var (
	dir   = flag.String("dir", "testdata", "base directory for all file paths")
	out   = flag.String("out", "output.bun.par2", "output filename relative to -dir")
	parse = flag.String("parse", "", "index .par2 file relative to -dir")
)

var manifest = bundle.ManifestInput{
	Name:  "manifest.json",
	Bytes: []byte(`{"version":1,"description":"reference bundle"}`),
}

func main() {
	flag.Parse()

	if *parse == "" {
		log.Fatalf("%s: args error: -parse flag is required", programName)
	}

	files := flag.Args()
	if len(files) == 0 {
		log.Fatalf("%s: args error: at least one input file must be given", programName)
	}

	fs := afero.NewOsFs()

	parsePath := filepath.Join(*dir, *parse)
	pf, err := par2.ParseFile(fs, parsePath, true)
	if err != nil {
		log.Fatalf("%s: parse error: %v", programName, err)
	}

	if len(pf.Sets) < 1 || pf.Sets[0].MainPacket == nil {
		log.Fatalf("%s: parsed file has no sets or main packet", programName)
	}

	recoverySetID := pf.Sets[0].MainPacket.SetID

	inputs := make([]bundle.FileInput, len(files))
	for i, name := range files {
		inputs[i] = bundle.FileInput{
			Name: filepath.Base(name),
			Path: filepath.Join(*dir, name),
		}
	}

	outPath := filepath.Join(*dir, *out)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil { //nolint:mnd
		log.Fatalf("%s: fs error: %v", programName, err)
	}

	if err := bundle.Pack(fs, outPath, recoverySetID, manifest, inputs); err != nil {
		log.Fatalf("%s: pack error: %v", programName, err)
	}

	bun, err := bundle.Open(fs, outPath)
	if err != nil {
		log.Fatalf("%s: bundle open error: %v", programName, err)
	}

	if err := bun.Validate(true); err != nil {
		bun.Close()
		log.Fatalf("%s: bundle validate error: %v", programName, err)
	}

	bun.Close()
	log.Printf("%s: success: %s\n", programName, outPath)
}
