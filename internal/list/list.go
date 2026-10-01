package list

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/desertwitch/par2cron/internal/verify"
	"github.com/spf13/afero"
)

type Options struct {
	SkipNotCreated bool   `json:"skip_not_created"`
	CacheDir       string `json:"cache_dir"`
}

type Service struct {
	fsys afero.Fs

	log     *logging.Logger
	logbase *logging.Logger
	runner  schema.CommandRunner
	walker  schema.FilesystemWalker
	bundler schema.BundleHandler
	cacher  schema.CacheHandler
}

func NewService(fsys afero.Fs, log *logging.Logger, runner schema.CommandRunner, bundler schema.BundleHandler, cacher schema.CacheHandler) *Service {
	var walker schema.FilesystemWalker
	if _, ok := fsys.(*afero.OsFs); ok {
		walker = util.OSWalker{}
	} else {
		walker = util.AferoWalker{Fs: fsys}
	}

	return &Service{
		fsys:    fsys,
		logbase: log,
		log:     log.With("op", "list"),
		runner:  runner,
		walker:  walker,
		bundler: bundler,
		cacher:  cacher,
	}
}

func (prog *Service) openCache(ctx context.Context, rootDir string, opts Options) schema.Cache {
	cache := prog.cacher.NewCache(prog.fsys, opts.CacheDir, rootDir)

	if opts.CacheDir == "" {
		return cache
	}

	if err := cache.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		logger := prog.listLogger(ctx, rootDir)
		logger.Error("Failed to load manifest cache", "error", err)
	}

	return cache
}

func (prog *Service) List(ctx context.Context, rootDirs []string, opts Options) error {
	if prog.log.Options.WantJSON {
		return prog.PrintJSON(ctx, rootDirs, opts)
	}

	logger := prog.listLogger(ctx, nil)

	vs := verify.NewService(prog.fsys, prog.logbase, prog.runner, prog.bundler, prog.cacher)
	va := verify.Options{IncludeExternal: false, SkipNotCreated: opts.SkipNotCreated}

	metas := []*verify.JobMeta{}
	for _, rootDir := range rootDirs {
		cache := prog.openCache(ctx, rootDir, opts)

		logger.Debug("Scanning filesystem for jobs...",
			"walker", prog.walker.Name(), "path", rootDir, "cached", cache.Len())

		meta, err := vs.Enumerate(ctx, rootDir, va, cache)
		if err != nil {
			if !errors.Is(err, schema.ErrNonFatal) {
				return fmt.Errorf("%s: failed to enumerate jobs: %w", rootDir, err)
			}

			logger.Error("Not all manifests could be read", "path", rootDir, "error", err)
		}

		cache.PruneUnwalked()
		// We don't save the cache so there cannot be races with verification.
		// A list could finish after an overlapping verification and discard
		// the verification progress in a race, so we only let verification
		// write to the cache (seeing a list does not mutate manifests anyway).

		metas = append(metas, meta...)
	}

	slices.SortFunc(metas, func(a, b *verify.JobMeta) int {
		return cmp.Or(
			cmp.Compare(statusOf(a), statusOf(b)),
			strings.Compare(a.Par2Path, b.Par2Path),
		)
	})

	return prog.printTable(metas)
}

func (prog *Service) printTable(metas []*verify.JobMeta) error {
	w := tabwriter.NewWriter(prog.log.Options.Stdout, 0, 0, 2, ' ', 0) //nolint:mnd

	fmt.Fprintln(w, "STATUS\tVERIFIED\tDURATION\tFAILURES\tCACHED\tPATH")
	for _, m := range metas {
		corrupt, verified, dur, cached := "-", "-", "-", "N"

		if m.Saved {
			cached = "Y"
		}
		if m.HasVerification {
			corrupt = strconv.Itoa(m.CountCorrupted)
			verified = m.VerifyTime.Local().Format("2006-01-02T15:04:05") //nolint:gosmopolitan
			dur = m.VerifyDuration.Round(time.Second).String()
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			statusOf(m), verified, dur, corrupt, cached, m.Par2Path)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to write list: %w", err)
	}

	return nil
}
