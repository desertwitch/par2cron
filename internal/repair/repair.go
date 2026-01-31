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

type Options struct {
	Par2Args             []string
	Par2Verify           bool
	MaxDuration          flags.Duration
	MinTestedCount       int
	SkipNotCreated       bool
	AttemptUnrepairables bool
	PurgeBackups         bool
	RestoreBackups       bool
}

type Service struct {
	fsys afero.Fs

	log    *logging.Logger
	runner schema.CommandRunner
	walker schema.FilesystemWalker
}

func NewService(fsys afero.Fs, log *logging.Logger, runner schema.CommandRunner) *Service {
	var walker schema.FilesystemWalker
	if _, ok := fsys.(*afero.OsFs); ok {
		walker = util.OSWalker{}
	} else {
		walker = util.AferoWalker{Fs: fsys}
	}

	return &Service{
		fsys:   fsys,
		log:    log,
		runner: runner,
		walker: walker,
	}
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

	manifest *schema.Manifest
}

func NewRepairJob(par2Path string, args Options, mf *schema.Manifest) *Job {
	rj := &Job{}

	rj.workingDir = filepath.Dir(par2Path)
	rj.par2Name = filepath.Base(par2Path)
	rj.par2Path = par2Path
	rj.par2Args = slices.Clone(args.Par2Args)
	rj.par2Verify = args.Par2Verify
	rj.manifestName = rj.par2Name + schema.ManifestExtension
	rj.manifestPath = rj.par2Path + schema.ManifestExtension
	rj.lockPath = rj.par2Path + schema.LockExtension
	rj.purgeBackups = args.PurgeBackups
	rj.restoreBackups = args.RestoreBackups

	rj.manifest = mf

	return rj
}

func (prog *Service) Repair(ctx context.Context, rootDir string, args Options) error {
	errs := []error{}

	logger := prog.repairLogger(ctx, nil, rootDir)
	logger.Info("Scanning filesystem for jobs...")

	jobs, err := prog.Enumerate(ctx, rootDir, args)
	if err != nil {
		if !errors.Is(err, schema.ErrNonFatal) {
			return fmt.Errorf("failed to enumerate jobs: %w", err)
		}

		err = fmt.Errorf("failed to enumerate some jobs: %w", err)
		errs = append(errs, fmt.Errorf("%w: %w", schema.ErrExitPartialFailure, err))
	}

	results := util.NewResultTracker(logger)
	defer results.PrintCompletionInfo(len(jobs))

	if len(jobs) > 0 {
		logger.Info(fmt.Sprintf("Starting to process %d jobs...", len(jobs)),
			"maxDuration", args.MaxDuration.Value.String())
	} else {
		logger.Info("Nothing to do (will check again next run)")
	}

	var deadlineCtx context.Context //nolint:contextcheck
	var deadlineCancel context.CancelFunc
	if args.MaxDuration.Value > 0 {
		deadlineCtx, deadlineCancel = context.WithDeadline(ctx, time.Now().Add(args.MaxDuration.Value))
		defer deadlineCancel()
	}

	for i, job := range jobs {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context error: %w", err)
		}

		if deadlineCtx != nil {
			if err := deadlineCtx.Err(); errors.Is(err, context.DeadlineExceeded) {
				logger := prog.repairLogger(ctx, nil, nil)
				logger.Warn("Exceeded the --duration budget (will continue next run)",
					"unprocessedJobs", len(jobs)-i, "totalJobs", len(jobs),
					"maxDuration", args.MaxDuration.Value.String())

				break
			}
		}

		pos := fmt.Sprintf("%d/%d", i+1, len(jobs))
		ctx := context.WithValue(ctx, schema.PosKey, pos)

		logger := prog.repairLogger(ctx, job, nil)
		logger.Info("Job started")

		if err := prog.runRepair(ctx, job); err == nil {
			logger.Info("Job completed with success")
			results.Success++
		} else if errors.Is(err, schema.ErrFileIsLocked) || errors.Is(err, schema.ErrManifestMismatch) {
			logger.Warn("Job unavailable (will retry next run)", "error", err)
			results.Skipped++
		} else if !errors.Is(err, schema.ErrAlreadyExists) {
			logger.Error("Job failure (will retry next run)", "error", err)
			errs = append(errs, fmt.Errorf("%w: %w", schema.ErrExitPartialFailure, err))
			results.Error++
		}
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error: %w", err)
	}

	return util.HighestError(errs) //nolint:wrapcheck
}

func (prog *Service) Enumerate(ctx context.Context, rootDir string, args Options) ([]*Job, error) {
	jobs := []*Job{}
	chkr := util.NewIgnoreChecker(prog.fsys)

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
		if skip, err := chkr.ShouldSkip(par2path, d.IsDir()); skip {
			logger := prog.repairLogger(ctx, nil, par2path)
			logger.Debug("A path was skipped due to a present ignore-file", "error", err)

			return err //nolint:wrapcheck
		}

		if !util.IsPar2Base(par2path) {
			return nil
		}

		job, err := prog.processManifest(ctx, par2path, args)
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

func (prog *Service) processManifest(ctx context.Context, par2path string, args Options) (*Job, error) {
	manifestPath := par2path + schema.ManifestExtension
	logger := prog.repairLogger(ctx, nil, manifestPath)

	if _, err := prog.fsys.Stat(manifestPath); err != nil {
		logger.Debug("Failed to find par2cron manifest (will retry next run)", "error", err)

		return nil, schema.ErrSilentSkip
	}

	unlock, err := util.AcquireLock(prog.fsys, par2path+schema.LockExtension, false)
	if err != nil {
		if errors.Is(err, schema.ErrFileIsLocked) {
			logger.Debug("Manifest is locked by another instance (will retry next run)")

			return nil, schema.ErrSilentSkip
		}

		return nil, fmt.Errorf("failed to lock: %w", err)
	}
	data, err := afero.ReadFile(prog.fsys, manifestPath)
	if err != nil {
		logger.Error("Failed to read par2cron manifest (will retry next run)", "error", err)
		unlock()

		return nil, schema.ErrNonFatal
	}
	unlock()

	mf := &schema.Manifest{}
	if err := json.Unmarshal(data, mf); err != nil {
		logger.Warn("Failed to unmarshal par2cron manifest (will retry next run)", "error", err)

		return nil, schema.ErrSilentSkip
	}

	if args.SkipNotCreated && mf.Creation == nil {
		logger.Debug("No creation manifest (skipping; --skip-not-created)")

		return nil, schema.ErrSilentSkip
	}

	if mf.Verification == nil {
		logger.Debug("No verification manifest (skipping; not a repair candidate)")

		return nil, schema.ErrSilentSkip
	}

	if mf.Verification.RepairNeeded && (mf.Verification.CountCorrupted >= args.MinTestedCount) {
		if args.AttemptUnrepairables || mf.Verification.RepairPossible {
			return NewRepairJob(par2path, args, mf), nil
		}
	}

	logger.Debug("Not a candidate for repair",
		"minTested", args.MinTestedCount,
		"actualTested", mf.Verification.CountCorrupted,
		"repairNeeded", mf.Verification.RepairNeeded,
		"repairPossible", mf.Verification.RepairPossible,
	)

	return nil, schema.ErrSilentSkip
}

//nolint:funlen
func (prog *Service) runRepair(ctx context.Context, job *Job) error {
	logger := prog.repairLogger(ctx, job, job.par2Path)

	unlock, err := util.AcquireLock(prog.fsys, job.lockPath, false)
	if err != nil {
		return fmt.Errorf("failed to lock: %w", err)
	}
	defer unlock()

	par2Hash, err := util.HashFile(prog.fsys, job.par2Path)
	if err != nil {
		logger.Error("Failed to hash PAR2 against par2cron manifest", "error", err)

		return fmt.Errorf("failed to hash par2: %w", err)
	}

	if par2Hash != job.manifest.SHA256 {
		logger.Warn("PAR2 has changed (needs re-verification; skipping repair)",
			"currentHash", par2Hash,
			"manifestHash", job.manifest.SHA256,
		)

		return fmt.Errorf("%w: par2 hash mismatch", schema.ErrManifestMismatch)
	}

	cmdArgs := make([]string, 0, 1+len(job.par2Args)+1+1)
	cmdArgs = append(cmdArgs, "repair")
	cmdArgs = append(cmdArgs, job.par2Args...)
	cmdArgs = append(cmdArgs, "--")
	cmdArgs = append(cmdArgs, job.par2Path)

	if job.manifest.Repair == nil {
		job.manifest.Repair = &schema.RepairManifest{}
	}
	job.manifest.Repair.Args = slices.Clone(job.par2Args)
	job.manifest.Repair.Count++

	var purger *backupPurger
	if job.purgeBackups {
		purger, err = newBackupPurger(prog.fsys, prog.repairLogger(ctx, job, nil), job.workingDir)
		if err != nil {
			logger.Warn("Failed to create backup file purger (cannot --purge-backups)",
				"error", err)
		}
	}

	var needsRestore bool
	if job.restoreBackups {
		restorer, err := newBackupRestorer(prog.fsys, prog.repairLogger(ctx, job, nil), job.workingDir)
		if err != nil {
			logger.Warn("Failed to create backup file restorer (cannot --restore-backups)", "error", err)
		} else {
			defer func() {
				if needsRestore {
					if err := restorer.Restore(); err != nil {
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
		logger.Error("Failed to repair PAR2", "error", err)

		return err
	}

	job.manifest.Repair.ExitCode = schema.Par2ExitCodeSuccess

	// if job.manifest.Par2Data == nil {
	// 	util.Par2IndexToManifest(prog.fsys, util.Par2IndexToManifestOptions{
	// 		Time:     job.manifest.Repair.Time,
	// 		Path:     job.par2Path,
	// 		Manifest: job.manifest,
	// 	}, logger)
	// }

	if err := util.WriteManifest(prog.fsys, job.manifestPath, job.manifest); err != nil {
		logger := prog.repairLogger(ctx, job, job.manifestPath)
		logger.Warn("Failed to write par2cron manifest (will retry on verify)", "error", err)
	}

	if job.par2Verify {
		vs := verify.NewService(prog.fsys, prog.log, prog.runner)
		vj := verify.NewJob(job.par2Path, verify.Options{}, job.manifest)

		if err := vs.RunVerify(ctx, vj, true); err != nil {
			return fmt.Errorf("failed to verify par2: %w", err)
		}
	}

	if purger != nil && job.purgeBackups {
		if err := purger.Purge(); err != nil {
			logger.Warn("Failed to remove backup files (cannot --purge-backups)",
				"error", err)
		}
	}

	return nil
}
