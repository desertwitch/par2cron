package tool

import (
	"context"
	"errors"
	"fmt"

	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
)

func (prog *Service) OutputMD5(ctx context.Context, paths []string) error {
	var errs []error
	seen := make(map[par2.Hash]bool)

	for _, path := range paths {
		f, err := prog.par2er.ParseFile(prog.fsys, path, false)
		if err != nil {
			logger := prog.toolLogger(ctx, path)
			logger.Error("Failed to parse PAR2 file", "error", err)

			errs = append(errs, fmt.Errorf("%s: %w", path, err))

			continue
		}

		for _, set := range f.Sets {
			for _, fp := range set.RecoverySet {
				if seen[fp.FileID] {
					continue
				}

				seen[fp.FileID] = true
				fmt.Fprintf(prog.log.Options.Stdout, "%x  %s\n", fp.Hash, fp.Name)
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%w: %d/%d failed: %w",
			schema.ErrExitPartialFailure, len(errs), len(paths), errors.Join(errs...))
	}

	return nil
}
