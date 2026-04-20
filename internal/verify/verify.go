package verify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/desertwitch/par2cron/internal/flags"
	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/spf13/afero"
)

const (
	prioNoManifest     = 0
	prioNoVerification = 1
	prioNeedsRepair    = 2
	prioOther          = 3
)

var _ schema.OptionsPar2ArgsSettable = (*Options)(nil)

type Options struct {
	Par2Args        []string
	MinAge          flags.Duration
	MaxDuration     flags.Duration
	RunInterval     flags.Duration
	IncludeExternal bool
	SkipNotCreated  bool
}

func (o *Options) SetPar2Args(args []string) {
	o.Par2Args = slices.Clone(args)
}

type Job struct {
	workingDir   string
	par2Name     string
	par2Path     string
	par2Args     []string
	manifestName string
	manifestPath string
	lockPath     string

	isBundle bool
	manifest *schema.Manifest
}

func NewJob(par2Path string, opts Options, mf *schema.Manifest, isBundle bool) *Job {
	vj := &Job{}

	vj.workingDir = filepath.Dir(par2Path)
	vj.par2Name = filepath.Base(par2Path)
	vj.par2Path = par2Path
	vj.par2Args = slices.Clone(opts.Par2Args)

	if !isBundle {
		vj.manifestName = vj.par2Name + schema.ManifestExtension
		vj.manifestPath = vj.par2Path + schema.ManifestExtension
		vj.lockPath = vj.par2Path + schema.LockExtension
	} else {
		vj.manifestName = vj.par2Name
		vj.manifestPath = vj.par2Path
		vj.lockPath = vj.par2Path
	}

	vj.isBundle = isBundle
	vj.manifest = mf

	return vj
}

type Service struct {
	fsys afero.Fs

	log    *logging.Logger
	runner schema.CommandRunner
	walker schema.FilesystemWalker
	bundler schema.BundleHandler
}

func NewService(fsys afero.Fs, log *logging.Logger, runner schema.CommandRunner, bundler schema.BundleHandler) *Service {
	var walker schema.FilesystemWalker
	if _, ok := fsys.(*afero.OsFs); ok {
		walker = util.OSWalker{}
	} else {
		walker = util.AferoWalker{Fs: fsys}
	}

	return &Service{
		fsys:   fsys,
		log:    log.With("op", "verify"),
		runner: runner,
		walker: walker,
		bundler: bundler,
	}
}

//nolint:funlen
func (prog *Service) Verify(ctx context.Context, rootDirs []string, opts Options) (util.ResultTracker, error) {
	errs := []error{}
	results := util.NewResultTracker()
	logger := prog.verificationLogger(ctx, nil, nil)

	jobs := []*Job{}
	for _, rootDir := range rootDirs {
		logger.Info("Scanning filesystem for jobs...",
			"walker", prog.walker.Name(), "path", rootDir)

		js, err := prog.Enumerate(ctx, rootDir, opts)
		if err != nil {
			if !errors.Is(err, schema.ErrNonFatal) {
				return results, fmt.Errorf("failed to enumerate jobs: %w", err)
			}

			err = fmt.Errorf("failed to enumerate some jobs: %w", err)
			errs = append(errs, fmt.Errorf("%w: %w", schema.ErrExitPartialFailure, err))
		}

		jobs = append(jobs, js...)
	}

	jobs = filterByAge(jobs, opts.MinAge.Value)
	sortJobs(jobs)
	prog.considerBacklog(jobs, opts)
	jobs = filterByDuration(jobs, opts.MaxDuration.Value)

	if len(jobs) > 0 {
		logger.Info(fmt.Sprintf("Starting to process %d jobs...", len(jobs)),
			"maxDuration", opts.MaxDuration.Value.String())
		results.Selected = len(jobs)
	} else {
		logger.Info("Nothing to do (will check again next run)",
			"minAge", opts.MinAge.Value.String())
	}

	prog.considerDurations(jobs, opts)

	var deadlineCtx context.Context //nolint:contextcheck
	var deadlineCancel context.CancelFunc
	if opts.MaxDuration.Value > 0 {
		deadlineCtx, deadlineCancel = context.WithDeadline(ctx, time.Now().Add(opts.MaxDuration.Value))
		defer deadlineCancel()
	}

	for i, job := range jobs {
		if err := ctx.Err(); err != nil {
			return results, fmt.Errorf("context error: %w", err)
		}

		if deadlineCtx != nil {
			if err := deadlineCtx.Err(); errors.Is(err, context.DeadlineExceeded) {
				logger := prog.verificationLogger(ctx, nil, nil)
				logger.Warn("Exceeded the --duration budget (will continue next run)",
					"unprocessedJobs", len(jobs)-i, "totalJobs", len(jobs),
					"maxDuration", opts.MaxDuration.Value.String())

				break
			}
		}

		pos := fmt.Sprintf("%d/%d", i+1, len(jobs))
		prio := job.queuePriority()

		ctx := context.WithValue(ctx, schema.PosKey, pos)
		ctx = context.WithValue(ctx, schema.PrioKey, prio)

		logger := prog.verificationLogger(ctx, job, nil)
		logger.Info("Job started", "estDuration", job.lastDurationStr())

		if err := prog.RunVerify(ctx, job, false); err == nil {
			if job.manifest.Verification.ExitCode == schema.Par2ExitCodeSuccess {
				logger.Info("Job completed with success",
					"runDuration", job.manifest.Verification.Duration.String(),
					"exitCode", job.manifest.Verification.ExitCode,
					"repairNeeded", job.manifest.Verification.RepairNeeded,
					"repairPossible", job.manifest.Verification.RepairPossible,
				)
				results.Success++
			} else {
				logger.Error("Job completed with corruption detected",
					"runDuration", job.manifest.Verification.Duration.String(),
					"exitCode", job.manifest.Verification.ExitCode,
					"repairNeeded", job.manifest.Verification.RepairNeeded,
					"repairPossible", job.manifest.Verification.RepairPossible,
				)

				if job.manifest.Verification.RepairPossible {
					errs = append(errs, schema.ErrExitRepairable)
				} else {
					errs = append(errs, schema.ErrExitUnrepairable)
				}
				results.Error++
			}
		} else if errors.Is(err, schema.ErrFileIsLocked) {
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

func (prog *Service) Enumerate(ctx context.Context, rootDir string, opts Options) ([]*Job, error) {
	jobs := []*Job{}

	var partialErrors int
	err := prog.walker.WalkDir(rootDir, func(par2path string, d fs.DirEntry, err error) error {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context error: %w", err)
		}
		if err != nil {
			logger := prog.verificationLogger(ctx, nil, par2path)
			logger.Warn("A path was skipped due to FS error (will retry next run)", "error", err)

			return nil
		}

		if !util.IsPar2Index(d.Name()) {
			return nil
		}
		if util.ShouldIgnorePath(prog.fsys, par2path, rootDir) {
			logger := prog.verificationLogger(ctx, nil, par2path)
			logger.Debug("A path was skipped due to a present ignore-file")

			return nil
		}

		job, err := prog.processManifest(ctx, par2path, opts)
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

func (prog *Service) processManifest(ctx context.Context, par2path string, opts Options) (*Job, error) { //nolint:funcorder
	if strings.Contains(filepath.Base(par2path), schema.BundleExtension) {
		return prog.processBundleManifest(ctx, par2path, opts)
	}

	manifestPath := par2path + schema.ManifestExtension
	logger := prog.verificationLogger(ctx, nil, manifestPath)

	if _, err := util.LstatIfPossible(prog.fsys, manifestPath); err != nil {
		if !opts.IncludeExternal {
			logger.Debug("No manifest found (skipping)")

			return nil, schema.ErrSilentSkip
		}

		job := NewJob(par2path, opts, nil, false)

		logger := prog.verificationLogger(ctx, job, manifestPath)
		logger.Debug("Failed to find par2cron manifest (resetting manifest)", "error", err)

		return job, nil
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
		if opts.SkipNotCreated {
			logger.Debug("No unmarshalable manifest (skipping; --skip-not-created)")

			return nil, schema.ErrSilentSkip
		}

		job := NewJob(par2path, opts, nil, false)

		logger := prog.verificationLogger(ctx, job, manifestPath)
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

func (prog *Service) processBundleManifest(ctx context.Context, bundlePath string, opts Options) (*Job, error) { //nolint:funcorder
	logger := prog.verificationLogger(ctx, nil, bundlePath)

	unlock, err := util.AcquireLock(prog.fsys, bundlePath, false)
	if err != nil {
		if errors.Is(err, schema.ErrFileIsLocked) {
			logger.Debug("Bundle is locked by another instance (will retry next run)")

			return nil, schema.ErrSilentSkip
		}

		return nil, fmt.Errorf("failed to lock: %w", err)
	}
	b, err := prog.bundler.Open(prog.fsys, bundlePath)
	if err != nil {
		unlock()
		logger.Error("Failed to open bundle (will retry next run)", "error", err)

		return nil, schema.ErrNonFatal
	}
	by, err := b.Manifest()
	if err != nil {
		job := NewJob(bundlePath, opts, nil, true)

		logger := prog.verificationLogger(ctx, job, bundlePath)
		logger.Warn("Failed to read par2cron manifest (resetting manifest)", "error", err)

		_ = b.Close()
		unlock()

		return job, nil
	}
	_ = b.Close()
	unlock()

	mf := &schema.Manifest{}
	if err := json.Unmarshal(by, mf); err != nil {
		if opts.SkipNotCreated {
			logger.Debug("No unmarshalable manifest (skipping; --skip-not-created)")

			return nil, schema.ErrSilentSkip
		}

		job := NewJob(bundlePath, opts, nil, true)

		logger := prog.verificationLogger(ctx, job, bundlePath)
		logger.Warn("Failed to unmarshal par2cron manifest (resetting manifest)", "error", err)

		return job, nil
	}

	if opts.SkipNotCreated && mf.Creation == nil {
		logger.Debug("No creation manifest (skipping; --skip-not-created)")

		return nil, schema.ErrSilentSkip
	}

	job := NewJob(bundlePath, opts, mf, true)

	return job, nil
}

func (prog *Service) RunVerify(ctx context.Context, job *Job, isPreLocked bool) error {
	logger := prog.verificationLogger(ctx, job, job.manifestPath)

	if !isPreLocked {
		unlock, err := util.AcquireLock(prog.fsys, job.lockPath, false)
		if err != nil {
			return fmt.Errorf("failed to lock: %w", err)
		}
		defer unlock()
	}

	var par2Hash string
	if !job.isBundle {
		hash, err := util.HashFile(prog.fsys, job.par2Path)
		if err != nil {
			logger.Error("Failed to hash PAR2 against par2cron manifest", "error", err)

			return fmt.Errorf("failed to hash par2: %w", err)
		}
		par2Hash = hash

		if job.manifest != nil && par2Hash != job.manifest.SHA256 {
			logger.Warn("PAR2 has changed (manifest out of date; resetting manifest)",
				"currentHash", par2Hash,
				"manifestHash", job.manifest.SHA256,
			)

			job.manifest = nil
		}
	}

	if job.manifest == nil {
		job.manifest = schema.NewManifest(job.par2Name)
		job.manifest.SHA256 = par2Hash
	}

	if job.manifest.Verification == nil {
		job.manifest.Verification = schema.NewVerificationManifest()
	}
	job.manifest.Verification.ProgramVersion = schema.ProgramVersion
	job.manifest.Verification.Par2Version = schema.Par2Version
	job.manifest.Verification.Args = slices.Clone(job.par2Args)

	cmdArgs := make([]string, 0, 1+len(job.par2Args)+1+1)
	cmdArgs = append(cmdArgs, "verify")
	cmdArgs = append(cmdArgs, job.par2Args...)
	cmdArgs = append(cmdArgs, "--")
	cmdArgs = append(cmdArgs, job.par2Path)

	job.manifest.Verification.Time = time.Now()
	err := prog.runner.Run(ctx, "par2", cmdArgs, job.workingDir, prog.log.Options.Stdout, prog.log.Options.Stdout)
	job.manifest.Verification.Duration = time.Since(job.manifest.Verification.Time)

	if err := prog.parseExitCode(job, err); err != nil {
		err = fmt.Errorf("par2cmdline: %w", err)

		logger := prog.verificationLogger(ctx, job, job.par2Path)
		logger.Error("Failed to verify PAR2", "error", err)

		return err
	}

	job.manifest.Verification.Count++

	// if job.manifest.Par2Data == nil {
	// 	util.Par2ToManifest(prog.fsys, util.Par2ToManifestOptions{
	// 		Time:     job.manifest.Verification.Time,
	// 		Path:     job.par2Path,
	// 		Manifest: job.manifest,
	// 	}, prog.verificationLogger(ctx, job, nil))
	// }

	if err := util.WriteManifest(prog.fsys, prog.bundler, job.manifestPath, job.manifest, job.isBundle); err != nil {
		logger := prog.verificationLogger(ctx, job, job.manifestPath)
		logger.Error("Failed to write par2cron manifest", "error", err)

		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}

func (prog *Service) parseExitCode(job *Job, err error) error {
	if err == nil {
		job.manifest.Verification.ExitCode = 0
	} else {
		c := util.AsExitCode(err)
		if c == nil {
			return err // No exit code to parse, return the error.
		}

		job.manifest.Verification.ExitCode = *c
		err = fmt.Errorf("%w (%d)", err, *c)
	}

	switch job.manifest.Verification.ExitCode {
	case schema.Par2ExitCodeSuccess:
		job.manifest.Verification.RepairNeeded = false
		job.manifest.Verification.RepairPossible = true
		job.manifest.Verification.CountCorrupted = 0

		return nil

	case schema.Par2ExitCodeRepairPossible:
		job.manifest.Verification.RepairNeeded = true
		job.manifest.Verification.RepairPossible = true
		job.manifest.Verification.CountCorrupted++

		return nil

	case schema.Par2ExitCodeRepairImpossible:
		job.manifest.Verification.RepairNeeded = true
		job.manifest.Verification.RepairPossible = false
		job.manifest.Verification.CountCorrupted++

		return nil

	default:
		return err // Unhandled exit code, return the error.
	}
}

func (prog *Service) considerBacklog(jobs []*Job, opts Options) {
	if len(jobs) == 0 || opts.MinAge.Value <= 0 || opts.MaxDuration.Value <= 0 || opts.RunInterval.Value <= 0 {
		return
	}

	js := prog.Stats(jobs)
	if js.TotalDuration <= 0 {
		return
	}

	runsPerCycle := max(int(opts.MinAge.Value/opts.RunInterval.Value), 1)
	capacity := time.Duration(runsPerCycle) * opts.MaxDuration.Value

	if js.TotalDuration > capacity {
		prog.log.Warn("Backlog is growing indefinitely (increase --age, increase --duration, "+
			"or verify without --duration once to clear the backlog and then fix your arguments)",
			"totalDuration", js.TotalDuration.String(),
			"clearingCapacity", capacity.String(),
			"clearingShortfall", (js.TotalDuration - capacity).String(),
		)
	}
}

func (prog *Service) considerDurations(jobs []*Job, opts Options) {
	if len(jobs) == 0 {
		return
	}

	if opts.MaxDuration.Value > 0 {
		est := jobs[0].lastDuration()
		switch {
		case est == 0:
			prog.log.Warn("First job has (still) unknown duration, may exceed --duration",
				"job", jobs[0].par2Path,
				"maxDuration", opts.MaxDuration.Value.String(),
			)
		case est > opts.MaxDuration.Value:
			prog.log.Warn("First job is estimated to exceed --duration (required to prevent starvation)",
				"job", jobs[0].par2Path,
				"estDuration", est.String(),
				"maxDuration", opts.MaxDuration.Value.String(),
			)
		}

		for _, job := range jobs[1:] {
			if job.lastDuration() == 0 {
				prog.log.Warn("Some jobs have a (still) unknown duration, may exceed --duration",
					"maxDuration", opts.MaxDuration.Value.String(),
				)

				break
			}
		}
	}
}
