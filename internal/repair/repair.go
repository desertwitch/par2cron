package repair

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"time"

	"github.com/desertwitch/par2cron/internal/flags"
	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/desertwitch/par2cron/internal/verify"
	"github.com/spf13/afero"
)

var _ schema.OptionsPar2ArgsSettable = (*Options)(nil)

type Options struct {
	Par2Args             []string
	Par2Verify           bool
	MaxDuration          flags.Duration
	MinTestedCount       int
	SkipNotCreated       bool
	AttemptUnrepairables bool
	PurgeBackups         bool
	RestoreBackups       bool
	CacheDir             string
}

func (o *Options) SetPar2Args(args []string) {
	o.Par2Args = slices.Clone(args)
}

type Service struct {
	fsys afero.Fs

	log     *logging.Logger
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
		log:     log.With("op", "repair"),
		runner:  runner,
		walker:  walker,
		bundler: bundler,
		cacher:  cacher,
	}
}

type JobMeta struct {
	*schema.JobMeta
}

func NewJobMeta(meta *schema.JobMeta) *JobMeta {
	return &JobMeta{meta}
}

type Job struct {
	workingDir     string
	par2Name       string
	par2Path       string
	par2Args       []string
	par2Verify     bool
	manifestName   string
	manifestPath   string
	lockPath       string
	purgeBackups   bool
	restoreBackups bool

	isBundle bool
	manifest *schema.Manifest
}

func NewJob(par2Path string, opts Options, mf *schema.Manifest, isBundle bool) *Job {
	rj := &Job{}

	rj.workingDir = filepath.Dir(par2Path)
	rj.par2Name = filepath.Base(par2Path)
	rj.par2Path = par2Path
	rj.par2Args = slices.Clone(opts.Par2Args)
	rj.par2Verify = opts.Par2Verify

	if !isBundle {
		rj.manifestName = rj.par2Name + schema.ManifestExtension
		rj.manifestPath = rj.par2Path + schema.ManifestExtension
		rj.lockPath = rj.par2Path + schema.LockExtension
	} else {
		rj.manifestName = rj.par2Name
		rj.manifestPath = rj.par2Path
		rj.lockPath = rj.par2Path
	}

	rj.purgeBackups = opts.PurgeBackups
	rj.restoreBackups = opts.RestoreBackups

	rj.isBundle = isBundle
	rj.manifest = mf

	return rj
}

func (prog *Service) openCache(ctx context.Context, rootDir string, opts Options) schema.Cache {
	cache := prog.cacher.NewCache(prog.fsys, opts.CacheDir, rootDir)

	if opts.CacheDir == "" {
		return cache
	}

	if err := cache.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		logger := prog.repairLogger(ctx, nil, rootDir)
		logger.Error("Failed to load manifest cache", "error", err)
	}

	return cache
}

func (prog *Service) Repair(ctx context.Context, rootDirs []string, opts Options) (util.ResultTracker, error) {
	errs := []error{}
	results := util.NewResultTracker()
	logger := prog.repairLogger(ctx, nil, nil)

	metas := []*JobMeta{}
	for _, rootDir := range rootDirs {
		cache := prog.openCache(ctx, rootDir, opts)

		logger.Info("Scanning filesystem for jobs...",
			"walker", prog.walker.Name(), "path", rootDir, "cached", cache.Len())

		ms, err := prog.Enumerate(ctx, rootDir, opts, cache)
		if err != nil {
			if !errors.Is(err, schema.ErrNonFatal) {
				return results, fmt.Errorf("failed to enumerate jobs: %w", err)
			}

			err = fmt.Errorf("failed to enumerate some jobs: %w", err)
			errs = append(errs, fmt.Errorf("%w: %w", schema.ErrExitPartialFailure, err))
		}

		cache.PruneUnwalked()
		// We don't save the cache so there cannot be races with verification.
		// A repair could finish after an overlapping verification and discard
		// the verification progress in a race, so we only let verification
		// write to the cache (it needs to confirm the repair results anyway).

		metas = append(metas, ms...)
	}

	if len(metas) > 0 {
		logger.Info(fmt.Sprintf("Starting to process %d jobs...", len(metas)),
			"maxDuration", opts.MaxDuration.Value.String())
		results.Selected = len(metas)
	} else {
		logger.Info("Nothing to do (will check again next run)")
	}

	var deadlineCtx context.Context //nolint:contextcheck
	var deadlineCancel context.CancelFunc
	if opts.MaxDuration.Value > 0 {
		deadlineCtx, deadlineCancel = context.WithDeadline(ctx, time.Now().Add(opts.MaxDuration.Value))
		defer deadlineCancel()
	}

	for i, meta := range metas {
		if err := ctx.Err(); err != nil {
			return results, fmt.Errorf("context error: %w", err)
		}

		if deadlineCtx != nil {
			if err := deadlineCtx.Err(); errors.Is(err, context.DeadlineExceeded) {
				logger := prog.repairLogger(ctx, nil, nil)
				logger.Warn("Exceeded the --duration budget (will continue next run)",
					"unprocessedJobs", len(metas)-i, "totalJobs", len(metas),
					"maxDuration", opts.MaxDuration.Value.String())

				break
			}
		}

		pos := fmt.Sprintf("%d/%d", i+1, len(metas))
		ctx := context.WithValue(ctx, schema.PosKey, pos)

		mf, err := prog.loadManifest(meta)
		if err != nil {
			if errors.Is(err, schema.ErrFileIsLocked) {
				logger.Warn("Manifest unavailable (will retry next run)", "error", err)
				results.Skipped++

				continue
			}

			logger.Error("Manifest failure (will retry next run)", "error", err)
			errs = append(errs, fmt.Errorf("%w: %w", schema.ErrExitPartialFailure, err))
			results.Error++

			continue
		}

		job := NewJob(meta.Par2Path, opts, mf, meta.IsBundle)
		logger := prog.repairLogger(ctx, job, nil)
		logger.Info("Job started")

		if err := prog.runRepair(ctx, job); err == nil {
			logger.Info("Job completed with success")
			results.Success++
		} else if errors.Is(err, schema.ErrFileIsLocked) || errors.Is(err, schema.ErrManifestMismatch) {
			logger.Warn("Job unavailable (will retry next run)", "error", err)
			results.Skipped++
		} else {
			logger.Error("Job failure (will retry next run)", "error", err)
			errs = append(errs, fmt.Errorf("%w: %w", schema.ErrExitPartialFailure, err))
			results.Error++
		}
	}

	if err := ctx.Err(); err != nil {
		return results, fmt.Errorf("context error: %w", err)
	}

	return results, util.HighestError(errs) //nolint:wrapcheck
}

func (prog *Service) Enumerate(ctx context.Context, rootDir string, opts Options, cache schema.Cache) ([]*JobMeta, error) {
	metas := []*JobMeta{}
	checker := util.NewIgnoreChecker(prog.fsys, rootDir)

	var partialErrors int
	err := prog.walker.WalkDir(rootDir, func(par2path string, d fs.DirEntry, err error) error {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context error: %w", err)
		}
		if err != nil {
			logger := prog.repairLogger(ctx, nil, par2path)
			logger.Warn("A path was skipped due to FS error (will retry next run)", "error", err)

			return nil
		}

		if !util.IsPar2Index(d.Name()) {
			return nil
		} // --- End of Hot Path ---
		if checker.ShouldIgnore(par2path) {
			logger := prog.repairLogger(ctx, nil, par2path)
			logger.Debug("A path was skipped due to a present ignore-file")

			return nil
		}

		if meta, cached := cache.Get(par2path); cached {
			if prog.isRepairCandidate(ctx, meta, opts) {
				metas = append(metas, NewJobMeta(meta))
			}
		} else {
			meta, err := prog.processManifest(ctx, par2path)
			if err != nil {
				if !errors.Is(err, schema.ErrNonFatal) && !errors.Is(err, schema.ErrSilentSkip) {
					return fmt.Errorf("failed to process manifest: %w", err)
				}
				if errors.Is(err, schema.ErrNonFatal) {
					partialErrors++
				}

				return nil
			}
			cache.Set(par2path, meta.JobMeta)

			if prog.isRepairCandidate(ctx, meta.JobMeta, opts) {
				metas = append(metas, meta)
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk FS: %w", err)
	}
	if partialErrors > 0 {
		return metas, fmt.Errorf("%w: %d manifests failed to read", schema.ErrNonFatal, partialErrors)
	}

	return metas, nil
}

func (prog *Service) isRepairCandidate(ctx context.Context, meta *schema.JobMeta, opts Options) bool {
	if opts.SkipNotCreated && !meta.HasCreation {
		logger := prog.repairLogger(ctx, meta, nil)
		logger.Debug("No creation manifest (skipping; --skip-not-created)")

		return false
	}

	if !meta.HasVerification {
		logger := prog.repairLogger(ctx, meta, nil)
		logger.Debug("No verification manifest (skipping; not a repair candidate)")

		return false
	}

	if meta.RepairNeeded && (meta.CountCorrupted >= opts.MinTestedCount) {
		if opts.AttemptUnrepairables || meta.RepairPossible {
			return true
		}
	}

	logger := prog.repairLogger(ctx, meta, nil)
	logger.Debug("Not a candidate for repair",
		"minTested", opts.MinTestedCount,
		"actualTested", meta.CountCorrupted,
		"repairNeeded", meta.RepairNeeded,
		"repairPossible", meta.RepairPossible,
	)

	return false
}

func (prog *Service) processManifest(ctx context.Context, par2path string) (*JobMeta, error) {
	if util.IsPar2Bundle(par2path) {
		return prog.processBundleManifest(ctx, par2path)
	}

	manifestPath := par2path + schema.ManifestExtension

	if _, err := util.LstatIfPossible(prog.fsys, manifestPath); err != nil {
		logger := prog.repairLogger(ctx, nil, manifestPath)
		logger.Debug("Failed to find par2cron manifest (will retry next run)", "error", err)

		return nil, schema.ErrSilentSkip
	}

	unlock, err := util.AcquireLock(prog.fsys, par2path+schema.LockExtension, false)
	if err != nil {
		if errors.Is(err, schema.ErrFileIsLocked) {
			logger := prog.repairLogger(ctx, nil, manifestPath)
			logger.Debug("Manifest is locked by another instance (will retry next run)")

			return nil, schema.ErrSilentSkip
		}

		return nil, fmt.Errorf("failed to lock: %w", err)
	}
	data, err := afero.ReadFile(prog.fsys, manifestPath)
	if err != nil {
		logger := prog.repairLogger(ctx, nil, manifestPath)
		logger.Error("Failed to read par2cron manifest (will retry next run)", "error", err)
		unlock()

		return nil, schema.ErrNonFatal
	}
	unlock()

	mf := &schema.Manifest{}
	if err := json.Unmarshal(data, mf); err != nil {
		logger := prog.repairLogger(ctx, nil, manifestPath)
		logger.Warn("Failed to unmarshal par2cron manifest (will retry next run)", "error", err)

		return nil, schema.ErrSilentSkip
	}

	return NewJobMeta(schema.NewJobMeta(par2path, mf, false)), nil
}

func (prog *Service) processBundleManifest(ctx context.Context, bundlePath string) (*JobMeta, error) {
	unlock, err := util.AcquireLock(prog.fsys, bundlePath, false)
	if err != nil {
		if errors.Is(err, schema.ErrFileIsLocked) {
			logger := prog.repairLogger(ctx, nil, bundlePath)
			logger.Debug("Bundle is locked by another instance (will retry next run)")

			return nil, schema.ErrSilentSkip
		}

		return nil, fmt.Errorf("failed to lock: %w", err)
	}
	bun, err := prog.bundler.Open(prog.fsys, bundlePath)
	if err != nil {
		unlock()
		logger := prog.repairLogger(ctx, nil, bundlePath)
		logger.Error("Failed to open bundle (will retry next run)", "error", err)

		return nil, schema.ErrNonFatal
	}
	by, err := bun.Manifest()
	if err != nil {
		_ = bun.Close()
		unlock()
		logger := prog.repairLogger(ctx, nil, bundlePath)
		logger.Error("Failed to read par2cron manifest (will retry next run)", "error", err)

		return nil, schema.ErrNonFatal
	}
	_ = bun.Close()
	unlock()

	mf := &schema.Manifest{}
	if err := json.Unmarshal(by, mf); err != nil {
		logger := prog.repairLogger(ctx, nil, bundlePath)
		logger.Error("Failed to unmarshal par2cron manifest (will retry next run)", "error", err)

		return nil, schema.ErrSilentSkip
	}

	return NewJobMeta(schema.NewJobMeta(bundlePath, mf, true)), nil
}

func (prog *Service) loadManifest(meta *JobMeta) (*schema.Manifest, error) {
	if meta.IsBundle {
		return prog.loadBundleManifest(meta)
	}

	manifestPath := meta.Par2Path + schema.ManifestExtension

	unlock, err := util.AcquireLock(prog.fsys, meta.Par2Path+schema.LockExtension, false)
	if err != nil {
		return nil, fmt.Errorf("failed to lock: %w", err)
	}
	data, err := afero.ReadFile(prog.fsys, manifestPath)
	if err != nil {
		unlock()

		return nil, fmt.Errorf("failed to read: %w", err)
	}
	unlock()

	mf := &schema.Manifest{}
	if err := json.Unmarshal(data, mf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	return mf, nil
}

func (prog *Service) loadBundleManifest(meta *JobMeta) (*schema.Manifest, error) {
	bundlePath := meta.Par2Path

	unlock, err := util.AcquireLock(prog.fsys, bundlePath, false)
	if err != nil {
		return nil, fmt.Errorf("failed to lock: %w", err)
	}
	bun, err := prog.bundler.Open(prog.fsys, bundlePath)
	if err != nil {
		unlock()

		return nil, fmt.Errorf("failed to open: %w", err)
	}
	by, err := bun.Manifest()
	if err != nil {
		_ = bun.Close()
		unlock()

		return nil, fmt.Errorf("failed to read: %w", err)
	}
	_ = bun.Close()
	unlock()

	mf := &schema.Manifest{}
	if err := json.Unmarshal(by, mf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	return mf, nil
}

//nolint:funlen
func (prog *Service) runRepair(ctx context.Context, job *Job) error {
	unlock, err := util.AcquireLock(prog.fsys, job.lockPath, false)
	if err != nil {
		return fmt.Errorf("failed to lock: %w", err)
	}
	defer unlock()

	if !job.isBundle {
		sha256hash, err := util.HashFile(prog.fsys, job.par2Path)
		if err != nil {
			logger := prog.repairLogger(ctx, job, job.par2Path)
			logger.Error("Failed to hash PAR2 against par2cron manifest", "error", err)

			return fmt.Errorf("failed to hash par2: %w", err)
		}

		if sha256hash != job.manifest.SHA256 {
			logger := prog.repairLogger(ctx, job, job.par2Path)
			logger.Warn("PAR2 has changed (needs re-verification; skipping repair)",
				"currentHash", sha256hash,
				"manifestHash", job.manifest.SHA256,
			)

			return fmt.Errorf("%w: par2 hash mismatch", schema.ErrManifestMismatch)
		}
	}

	cmdArgs := make([]string, 0, 1+len(job.par2Args)+1+1)
	cmdArgs = append(cmdArgs, "repair")
	cmdArgs = append(cmdArgs, job.par2Args...)
	cmdArgs = append(cmdArgs, "--")
	cmdArgs = append(cmdArgs, job.par2Path)

	if job.manifest.Repair == nil {
		job.manifest.Repair = schema.NewRepairManifest()
	}
	job.manifest.Repair.ProgramVersion = schema.ProgramVersion
	job.manifest.Repair.Par2Version = schema.Par2Version
	job.manifest.Repair.Args = slices.Clone(job.par2Args)
	job.manifest.Repair.Count++

	var purger *backupPurger
	if job.purgeBackups {
		purger, err = newBackupPurger(prog.fsys, prog.repairLogger(ctx, job, nil), job.workingDir)
		if err != nil {
			logger := prog.repairLogger(ctx, job, job.par2Path)
			logger.Warn("Failed to create backup file purger (cannot --purge-backups)",
				"error", err)
		}
	}

	var needsRestore bool
	if job.restoreBackups {
		restorer, err := newBackupRestorer(prog.fsys, prog.repairLogger(ctx, job, nil), job.workingDir)
		if err != nil {
			logger := prog.repairLogger(ctx, job, job.par2Path)
			logger.Warn("Failed to create backup file restorer (cannot --restore-backups)", "error", err)
		} else {
			defer func() {
				if needsRestore {
					if err := restorer.Restore(); err != nil {
						logger := prog.repairLogger(ctx, job, job.par2Path)
						logger.Warn("Failed to restore backup files (cannot --restore-backups)", "error", err)
					}
				}
			}()
		}
	}

	job.manifest.Repair.Time = time.Now()
	err = prog.runner.Run(ctx, "par2", cmdArgs, job.workingDir, prog.log.Options.Stdout, prog.log.Options.Stdout)
	job.manifest.Repair.Duration = time.Since(job.manifest.Repair.Time)

	if err != nil {
		needsRestore = true

		err = fmt.Errorf("par2cmdline: %w", err)
		c := util.AsExitCode(err)
		if c != nil {
			err = fmt.Errorf("%w (%d)", err, *c)
		}
		logger := prog.repairLogger(ctx, job, job.par2Path)
		logger.Error("Failed to repair PAR2", "error", err)

		return err
	}

	job.manifest.Repair.ExitCode = schema.Par2ExitCodeSuccess

	// if job.manifest.Par2Data == nil {
	// 	util.Par2ToManifest(prog.fsys, util.Par2ToManifestOptions{
	// 		Time:     job.manifest.Repair.Time,
	// 		Path:     job.par2Path,
	// 		Manifest: job.manifest,
	// 	}, prog.repairLogger(ctx, job, nil))
	// }

	if err := util.WriteManifest(prog.fsys, prog.bundler, job.manifestPath, job.manifest, job.isBundle); err != nil {
		logger := prog.repairLogger(ctx, job, job.manifestPath)
		logger.Warn("Failed to write par2cron manifest (will retry on verify)", "error", err)
	}

	if job.par2Verify {
		vs := verify.NewService(prog.fsys, prog.log, prog.runner, prog.bundler, prog.cacher)
		vj := verify.NewJob(job.par2Path, verify.Options{}, job.manifest, job.isBundle)

		if err := vs.RunVerify(ctx, vj, true); err != nil {
			return fmt.Errorf("failed to verify par2: %w", err)
		}
	}

	if purger != nil && job.purgeBackups {
		if err := purger.Purge(); err != nil {
			logger := prog.repairLogger(ctx, job, job.par2Path)
			logger.Warn("Failed to remove backup files (cannot --purge-backups)",
				"error", err)
		}
	}

	return nil
}
