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

	baseName := strings.TrimSuffix(job.par2Name, schema.Par2Extension) + "."
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

func getPaths(files []schema.FsElement) []string {
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}

	return paths
}
