package create

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/spf13/afero"
)

func (prog *Service) cleanupAfterFailure(ctx context.Context, job *Job) {
	entries, err := afero.ReadDir(prog.fsys, job.workingDir)
	if err != nil {
		logger := prog.creationLogger(ctx, job, job.workingDir)
		logger.Warn("Failed to read directory for cleanup (needs manual deletion)", "error", err)

		return
	}

	baseName := util.TrimSuffixFold(job.par2Name, schema.Par2Extension) + "."
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		if !strings.HasPrefix(name, baseName) {
			continue
		}
		if !util.IsPar2Index(name) && !util.IsPar2Volume(name) {
			continue
		}

		path := filepath.Join(job.workingDir, name)
		if err := prog.fsys.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			logger := prog.creationLogger(ctx, job, path)
			logger.Warn("Failed to cleanup a file after failure (needs manual deletion)", "error", err)
		}
	}

	for _, f := range []string{job.manifestPath, job.lockPath} {
		if err := prog.fsys.Remove(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
			logger := prog.creationLogger(ctx, job, f)
			logger.Warn("Failed to cleanup a file after failure (needs manual deletion)", "error", err)
		}
	}
}

func (prog *Service) considerRecursive(opts *Options) error {
	if opts.Par2Mode.Value != schema.CreateRecursiveMode && slices.Contains(opts.Par2Args, "-R") {
		prog.log.Error(
			"par2 default argument -R needs par2cron default --mode recursive (perhaps you meant -r, for redundancy?)",
			"error", errWrongModeArgument,
			"mode", opts.Par2Mode.Value,
			"args", opts.Par2Args,
		)

		return errWrongModeArgument
	}

	if opts.Par2Mode.Value == schema.CreateRecursiveMode && !slices.Contains(opts.Par2Args, "-R") {
		before := slices.Clone(opts.Par2Args)
		opts.Par2Args = append(opts.Par2Args, "-R")

		prog.log.Info(
			"Adding -R to par2 default arguments (due to --mode recursive)",
			"mode", opts.Par2Mode.Value,
			"args-before", before,
			"args-after", opts.Par2Args,
		)
	}

	return nil
}

func (prog *Service) par2AlreadyExists(ctx context.Context, job *Job) bool {
	baseName := util.TrimSuffixFold(job.par2Name, schema.Par2Extension)
	baseName = strings.TrimPrefix(baseName, ".")

	candidates := []string{
		// Lower-case variants
		filepath.Join(job.workingDir, baseName+schema.Par2Extension),
		filepath.Join(job.workingDir, "."+baseName+schema.Par2Extension),
		filepath.Join(job.workingDir, baseName+schema.BundleExtension+schema.Par2Extension),
		filepath.Join(job.workingDir, "."+baseName+schema.BundleExtension+schema.Par2Extension),

		// Upper-case variants
		filepath.Join(job.workingDir, baseName+strings.ToUpper(schema.Par2Extension)),
		filepath.Join(job.workingDir, "."+baseName+strings.ToUpper(schema.Par2Extension)),
		filepath.Join(job.workingDir, baseName+schema.BundleExtension+strings.ToUpper(schema.Par2Extension)),
		filepath.Join(job.workingDir, "."+baseName+schema.BundleExtension+strings.ToUpper(schema.Par2Extension)),
	}

	for _, path := range candidates {
		if _, err := util.LstatIfPossible(prog.fsys, path); err == nil {
			logger := prog.creationLogger(ctx, job, path)

			if job.markerPersist {
				logger.Debug("Same-named PAR2 already exists in folder (not overwriting)", "path", path)
			} else {
				logger.Warn("Same-named PAR2 already exists in folder (not overwriting)", "path", path)
			}

			return true
		}
	}

	return false
}

func getPaths(files []schema.FsElement) []string {
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}

	return paths
}
