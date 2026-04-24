package bundler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/desertwitch/par2cron/internal/bundle"
	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/spf13/afero"
)

type Options struct {
	Force           bool
	IncludeExternal bool
	SkipNotCreated  bool
}

type Service struct {
	fsys afero.Fs

	log     *logging.Logger
	walker  schema.FilesystemWalker
	bundler schema.BundleHandler
	par2er  schema.Par2Handler
}

func NewService(fsys afero.Fs, log *logging.Logger, bundler schema.BundleHandler, par2er schema.Par2Handler) *Service {
	var walker schema.FilesystemWalker
	if _, ok := fsys.(*afero.OsFs); ok {
		walker = util.OSWalker{}
	} else {
		walker = util.AferoWalker{Fs: fsys}
	}

	return &Service{
		fsys:    fsys,
		log:     log.With("op", "bundle"),
		walker:  walker,
		bundler: bundler,
		par2er:  par2er,
	}
}

type Job struct {
	par2Name     string
	par2Path     string
	manifestName string
	manifestPath string
	lockPath     string
	workingDir   string

	force    bool
	isBundle bool
	manifest *schema.Manifest
}

func NewJob(par2Path string, opts Options, mf *schema.Manifest, isBundle bool) *Job {
	bj := &Job{}

	bj.workingDir = filepath.Dir(par2Path)
	bj.par2Name = filepath.Base(par2Path)
	bj.par2Path = par2Path

	if !isBundle {
		bj.manifestName = bj.par2Name + schema.ManifestExtension
		bj.manifestPath = bj.par2Path + schema.ManifestExtension
		bj.lockPath = bj.par2Path + schema.LockExtension
	} else {
		bj.manifestName = bj.par2Name
		bj.manifestPath = bj.par2Path
		bj.lockPath = bj.par2Path
	}

	bj.force = opts.Force
	bj.isBundle = isBundle
	bj.manifest = mf

	return bj
}

type (
	enumFunc func(ctx context.Context, rootDir string, opts Options) ([]*Job, error)
	runFunc  func(ctx context.Context, job *Job) error
)

func (prog *Service) Pack(ctx context.Context, rootDirs []string, opts Options) (util.ResultTracker, error) {
	return prog.processMode(ctx, rootDirs, opts, prog.packEnumerate, prog.packBundle)
}

func (prog *Service) Unpack(ctx context.Context, rootDirs []string, opts Options) (util.ResultTracker, error) {
	return prog.processMode(ctx, rootDirs, opts, prog.unpackEnumerate, prog.unpackBundle)
}

func (prog *Service) processMode(ctx context.Context, rootDirs []string, opts Options, ef enumFunc, rf runFunc) (util.ResultTracker, error) {
	errs := []error{}
	results := util.NewResultTracker()
	logger := prog.bundleLogger(ctx, nil, nil)

	jobs := []*Job{}
	for _, rootDir := range rootDirs {
		logger.Info("Scanning filesystem for jobs...",
			"walker", prog.walker.Name(), "path", rootDir)

		js, err := ef(ctx, rootDir, opts)
		if err != nil {
			if !errors.Is(err, schema.ErrNonFatal) {
				return results, fmt.Errorf("failed to enumerate jobs: %w", err)
			}

			err = fmt.Errorf("failed to enumerate some jobs: %w", err)
			errs = append(errs, fmt.Errorf("%w: %w", schema.ErrExitPartialFailure, err))
		}

		jobs = append(jobs, js...)
	}

	if len(jobs) > 0 {
		logger.Info(fmt.Sprintf("Starting to process %d jobs...", len(jobs)))
		results.Selected = len(jobs)
	} else {
		logger.Info("Nothing to do (will check again next run)")
	}

	for i, job := range jobs {
		if err := ctx.Err(); err != nil {
			return results, fmt.Errorf("context error: %w", err)
		}

		pos := fmt.Sprintf("%d/%d", i+1, len(jobs))
		ctx := context.WithValue(ctx, schema.PosKey, pos)

		logger := prog.bundleLogger(ctx, job, nil)
		logger.Info("Job started")

		if err := rf(ctx, job); err == nil {
			logger.Info("Job completed with success")
			results.Success++
		} else {
			logger.Error("Job failure (skipping)", "error", err)
			errs = append(errs, fmt.Errorf("%w: %w", schema.ErrExitPartialFailure, err))
			results.Error++
		}
	}

	if err := ctx.Err(); err != nil {
		return results, fmt.Errorf("context error: %w", err)
	}

	return results, util.HighestError(errs) //nolint:wrapcheck
}

func (prog *Service) packEnumerate(ctx context.Context, rootDir string, opts Options) ([]*Job, error) {
	jobs := []*Job{}

	var partialErrors int
	err := prog.walker.WalkDir(rootDir, func(par2path string, d fs.DirEntry, err error) error {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context error: %w", err)
		}
		if err != nil {
			logger := prog.bundleLogger(ctx, nil, par2path)
			logger.Warn("A path was skipped due to FS error", "error", err)

			return nil
		}

		if !util.IsPar2Index(d.Name()) {
			return nil
		}
		if util.IsPar2Bundle(par2path) {
			return nil
		}
		if util.ShouldIgnorePath(prog.fsys, par2path, rootDir) {
			logger := prog.bundleLogger(ctx, nil, par2path)
			logger.Debug("A path was skipped due to a present ignore-file")

			return nil
		}

		job, err := prog.packProcessManifest(ctx, par2path, opts)
		if err != nil {
			if !errors.Is(err, schema.ErrNonFatal) && !errors.Is(err, schema.ErrSilentSkip) {
				return fmt.Errorf("failed to process manifest: %w", err)
			}
			if errors.Is(err, schema.ErrNonFatal) {
				partialErrors++
			}

			return nil
		}

		jobs = append(jobs, job)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk FS: %w", err)
	}
	if partialErrors > 0 {
		return jobs, fmt.Errorf("%w: %d manifests failed to read", schema.ErrNonFatal, partialErrors)
	}

	return jobs, nil
}

func (prog *Service) packProcessManifest(ctx context.Context, par2path string, opts Options) (*Job, error) {
	manifestPath := par2path + schema.ManifestExtension
	logger := prog.bundleLogger(ctx, nil, manifestPath)

	if _, err := util.LstatIfPossible(prog.fsys, manifestPath); err != nil {
		if !opts.IncludeExternal {
			logger.Debug("No manifest found (skipping)")

			return nil, schema.ErrSilentSkip
		}

		job := NewJob(par2path, opts, nil, false)

		logger := prog.bundleLogger(ctx, job, manifestPath)
		logger.Debug("Failed to find par2cron manifest (resetting manifest)", "error", err)

		return job, nil
	}

	unlock, err := util.AcquireLock(prog.fsys, par2path+schema.LockExtension, false)
	if err != nil {
		if errors.Is(err, schema.ErrFileIsLocked) {
			logger.Warn("Manifest is locked by another instance (skipping)")

			return nil, schema.ErrSilentSkip
		}

		return nil, fmt.Errorf("failed to lock: %w", err)
	}
	data, err := afero.ReadFile(prog.fsys, manifestPath)
	if err != nil {
		logger.Error("Failed to read par2cron manifest (skipping)", "error", err)
		unlock()

		return nil, schema.ErrNonFatal
	}
	unlock()

	mf := &schema.Manifest{}
	if err := json.Unmarshal(data, mf); err != nil {
		if opts.SkipNotCreated {
			logger.Debug("No unmarshalable manifest (skipping; --skip-not-created)")

			return nil, schema.ErrSilentSkip
		}

		job := NewJob(par2path, opts, nil, false)

		logger := prog.bundleLogger(ctx, job, manifestPath)
		logger.Warn("Failed to unmarshal par2cron manifest (resetting manifest)", "error", err)

		return job, nil
	}

	if opts.SkipNotCreated && mf.Creation == nil {
		logger.Debug("No creation manifest (skipping; --skip-not-created)")

		return nil, schema.ErrSilentSkip
	}

	job := NewJob(par2path, opts, mf, false)

	return job, nil
}

func (prog *Service) packBundle(ctx context.Context, job *Job) error {
	logger := prog.bundleLogger(ctx, job, nil)

	unlock, err := util.AcquireLock(prog.fsys, job.lockPath, false)
	if err != nil {
		return fmt.Errorf("failed to lock: %w", err)
	}
	defer unlock()

	files, err := util.FindBundleableFiles(prog.fsys, job.par2Name, job.workingDir)
	if err != nil {
		return fmt.Errorf("failed to find files to bundle: %w", err)
	}

	p, err := prog.par2er.ParseFile(prog.fsys, job.par2Path, true)
	if err != nil {
		return fmt.Errorf("failed to parse index par2: %w", err)
	}

	logger.Debug("Parsed PAR2 index file", "sets", len(p.Sets))
	if len(p.Sets) != 1 || p.Sets[0].MainPacket == nil {
		return errors.New("failed to parse index par2: malformed file")
	}

	recoverySetID := p.Sets[0].MainPacket.SetID
	logger.Debug("Parsed PAR2 main packet", "setID", recoverySetID)

	baseName := util.TrimSuffixFold(job.par2Name, schema.Par2Extension)
	bundleName := baseName + schema.BundleExtension + schema.Par2Extension
	bundlePath := filepath.Join(job.workingDir, bundleName)

	if job.manifest == nil {
		job.manifest = schema.NewManifest(job.par2Name)

		if sha256hash, err := util.HashFile(prog.fsys, job.par2Path); err == nil {
			job.manifest.SHA256 = sha256hash
		}
	}

	manifestData, err := json.MarshalIndent(job.manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	manifest := bundle.ManifestInput{
		Name:  job.manifestName,
		Bytes: manifestData,
	}

	if err := prog.bundler.Pack(prog.fsys, bundlePath, recoverySetID, manifest, files); err != nil {
		return fmt.Errorf("failed to pack bundle: %w", err)
	}

	for _, file := range files {
		if err := prog.fsys.Remove(file.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			logger := prog.bundleLogger(ctx, job, file.Path)
			logger.Warn("Failed to cleanup a file after bundling (needs manual deletion)", "error", err)
		}
	}

	for _, path := range []string{job.manifestPath, job.lockPath} {
		if err := prog.fsys.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			logger := prog.bundleLogger(ctx, job, path)
			logger.Warn("Failed to cleanup a file after bundling (needs manual deletion)", "error", err)
		}
	}

	return nil
}

func (prog *Service) unpackEnumerate(ctx context.Context, rootDir string, opts Options) ([]*Job, error) {
	jobs := []*Job{}

	err := prog.walker.WalkDir(rootDir, func(par2path string, d fs.DirEntry, err error) error {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context error: %w", err)
		}
		if err != nil {
			logger := prog.bundleLogger(ctx, nil, par2path)
			logger.Warn("A path was skipped due to FS error (will retry next run)", "error", err)

			return nil
		}

		if !util.IsPar2Index(d.Name()) {
			return nil
		}
		if !util.IsPar2Bundle(par2path) {
			return nil
		}
		if util.ShouldIgnorePath(prog.fsys, par2path, rootDir) {
			logger := prog.bundleLogger(ctx, nil, par2path)
			logger.Debug("A path was skipped due to a present ignore-file")

			return nil
		}

		jobs = append(jobs, NewJob(par2path, opts, nil, true))

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk FS: %w", err)
	}

	return jobs, nil
}

func (prog *Service) unpackBundle(ctx context.Context, job *Job) error {
	logger := prog.bundleLogger(ctx, job, nil)
	bundlePath := job.par2Path

	var cleanupPaths []string
	defer func() {
		for _, path := range cleanupPaths {
			if err := prog.fsys.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				logger := prog.bundleLogger(ctx, job, path)
				logger.Warn("Failed to cleanup a file after failure (needs manual removal)", "error", err)
			}
		}
	}()

	unlock, err := util.AcquireLock(prog.fsys, job.lockPath, false)
	if err != nil {
		return fmt.Errorf("failed to lock: %w", err)
	}
	defer unlock()

	bun, err := prog.bundler.Open(prog.fsys, bundlePath)
	if err != nil {
		return fmt.Errorf("failed to open bundle: %w", err)
	}
	defer bun.Close()

	if bun.IsRebuilt() {
		if !job.force {
			logger.Error("Bundle was rebuilt from corruption, complete unpack cannot be guaranteed (and --force is not set)")

			return errors.New("bundle is not guaranteed unpackable")
		}

		logger.Warn("Bundle was rebuilt from corruption, complete unpack cannot be guaranteed (but --force is set)")
	}

	files, err := bun.Unpack(prog.fsys, job.workingDir, false)
	if err != nil {
		if !onlyContains(err, bundle.ErrDataCorrupt) {
			cleanupPaths = append(cleanupPaths, files...)

			return fmt.Errorf("failed to unpack bundle: %w", err)
		}

		if !job.force {
			cleanupPaths = append(cleanupPaths, files...)
			logger.Error("Some files in the bundle are corrupted (and --force is not set)", "error", err)

			return fmt.Errorf("failed to unpack bundle: %w", err)
		}

		logger.Warn("Some files in the bundle are corrupted (but --force is set)", "error", err)
		logger.Warn("Not removing bundle file after unclean unpack (needs manual deletion)")
	} else {
		if err := prog.fsys.Remove(bundlePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			logger := prog.bundleLogger(ctx, job, bundlePath)
			logger.Warn("Failed to cleanup bundle file after unpacking (needs manual deletion)", "error", err)
		}
	}

	return nil
}

func onlyContains(err, sentinel error) bool {
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return errors.Is(err, sentinel)
	}

	for _, e := range joined.Unwrap() {
		if !errors.Is(e, sentinel) {
			return false
		}
	}

	return true
}
