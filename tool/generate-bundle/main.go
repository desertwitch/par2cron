package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/desertwitch/par2cron/internal/bundle"
	"github.com/spf13/afero"
)

var (
	dir = flag.String("dir", "testdata", "testdata directory")
	out = flag.String("out", "reference.bundle.par2", "output filename within -dir")
)

var recoverySetID = [16]byte{
	0xf3, 0x5c, 0x82, 0x41,
	0xc2, 0xfa, 0x13, 0x01,
	0x83, 0xc9, 0xdf, 0x6e,
	0xf3, 0x04, 0x62, 0x4b,
}

var manifest = bundle.ManifestInput{
	Name:  "manifest.json",
	Bytes: []byte(`{"version":1,"description":"reference bundle"}`),
}

var files = []string{
	"test.par2",
	"test.vol000+34.par2",
	"test.vol034+33.par2",
	"test.vol067+33.par2",
}

func main() {
	flag.Parse()

	inputs := make([]bundle.FileInput, len(files))
	for i, name := range files {
		inputs[i] = bundle.FileInput{
			Name: name,
			Path: filepath.Join(*dir, name),
		}
	}

	outPath := filepath.Join(*dir, *out)

	if err := bundle.Pack(afero.NewOsFs(), outPath, recoverySetID, manifest, inputs); err != nil {
		log.Fatalf("%s: error: %v", os.Args[0], err) //nolint:gosec
	}

	log.Printf("%s: success: %s\n", os.Args[0], outPath) //nolint:gosec
}
