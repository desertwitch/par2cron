package tool

import (
	"context"
	"errors"
	"fmt"

	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/util"
)

func (prog *Service) OutputMD5(ctx context.Context, paths []string, opts Options) error {
	var errs []error
	seen := make(map[par2.Hash]bool)

	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context error: %w", err)
		}
		if !opts.ParseAll && !util.IsPar2Index(path) {
			continue
		}

		var sets []par2.Set
		var bundleParsed bool

		if !opts.ParseAll && util.IsPar2Bundle(path) {
			bse, err := util.ParseBundlePar2Index(ctx, prog.fsys, path, prog.par2er, prog.bundler)
			if err != nil {
				logger := prog.toolLogger(ctx, path)
				logger.Error("Failed to parse PAR2 bundle", "error", err)

				errs = append(errs, fmt.Errorf("%s: %w", path, err))

				continue
			}

			sets = bse
			bundleParsed = true
		}

		if !bundleParsed {
			f, err := prog.par2er.ParseFile(prog.fsys, path, false)
			if err == nil {
				sets = f.Sets
			} else {
				logger := prog.toolLogger(ctx, path)
				logger.Error("Failed to parse PAR2 file", "error", err)

				errs = append(errs, fmt.Errorf("%s: %w", path, err))

				continue
			}
		}

		for _, set := range sets {
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
