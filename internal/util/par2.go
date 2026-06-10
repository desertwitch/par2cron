package util

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/spf13/afero"
)

const maxIndexSize = 100 * 1024 * 1024 // 100MiB

func ParseBundlePar2Index(fsys afero.Fs, path string, p schema.Par2Handler, b schema.BundleHandler) ([]par2.Set, error) {
	if !IsPar2Bundle(path) {
		return nil, errors.New("not a bundle file")
	}

	bun, err := b.Open(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("failed to open bundle: %w", err)
	}

	var sets []par2.Set
	for _, e := range bun.Entries() {
		if IsPar2Index(e.Name) {
			var buf bytes.Buffer

			if e.DataLength > maxIndexSize {
				return nil, errors.New("index file too large")
			}

			if err := bun.ExtractEntry(e, &buf); err != nil {
				return nil, fmt.Errorf("failed to extract index file: %w", err)
			}

			r := bytes.NewReader(buf.Bytes())

			s, err := p.Parse(r, true)
			if err != nil {
				return nil, fmt.Errorf("failed to parse index file: %w", err)
			}

			sets = append(sets, s...)
		}
	}

	if len(sets) > 0 {
		return sets, nil
	}

	return nil, errors.New("no index file found in bundle")
}
