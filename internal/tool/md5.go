package tool

import (
	"fmt"
	"io"

	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/spf13/afero"
)

func OutputMD5(paths []string, fsys afero.Fs, stdout io.Writer, stderr io.Writer, par2er schema.Par2Handler) error {
	var errors int
	seen := make(map[par2.Hash]bool)

	for _, path := range paths {
		f, err := par2er.ParseFile(fsys, path, false)
		if err != nil {
			errors++
			fmt.Fprintf(stderr, "%s: %v\n", path, err)

			continue
		}

		for _, set := range f.Sets {
			for _, fp := range set.RecoverySet {
				if seen[fp.FileID] {
					continue
				}

				seen[fp.FileID] = true
				fmt.Fprintf(stdout, "%x  %s\n", fp.Hash, fp.Name)
			}
		}
	}

	if errors > 0 {
		return fmt.Errorf("%w: %d files failed to parse",
			schema.ErrExitPartialFailure, errors)
	}

	return nil
}
