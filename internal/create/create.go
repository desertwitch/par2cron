package create

import (
	"context"
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
	"github.com/desertwitch/par2cron/internal/verify"
	"github.com/spf13/afero"
)

const (
	createMarkerPathPrefix    string = "_par2cron"
	createMarkerPathSeparator string = "_"
)

var (
	errNoFilesToProtect = errors.New("no files to protect")
	errSubjobFailure    = errors.New("subjob failure")
)

type Options struct {
	Par2Args    []string
	Par2Glob    string
	Par2Mode    flags.CreateMode
	Par2Verify  bool
	MaxDuration flags.Duration
	HideFiles   bool
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
	workingDir   string
	markerPath   string
	par2Mode     string
	par2Name     string
	par2Path     string
	par2Args     []string
	par2Glob     string
	par2Verify   bool
	lockPath     string
	manifestName string
	manifestPath string
}

func NewJob(markerPath string, cfg MarkerConfig) *Job {
	cj := &Job{}

	cj.par2Name = *cfg.Par2Name
	if *cfg.HideFiles && !strings.HasPrefix(cj.par2Name, ".") {
		cj.par2Name = "." + cj.par2Name
	}

	cj.par2Mode = cfg.Par2Mode.Value
	cj.par2Args = slices.Clone(*cfg.Par2Args)
	cj.par2Glob = *cfg.Par2Glob
	cj.par2Verify = *cfg.Par2Verify

	cj.markerPath = markerPath
	cj.workingDir = filepath.Dir(markerPath)
	cj.par2Path = filepath.Join(cj.workingDir, cj.par2Name)
	cj.lockPath = cj.par2Path + schema.LockExtension
	cj.manifestName = cj.par2Name + schema.ManifestExtension
	cj.manifestPath = cj.par2Path + schema.ManifestExtension

	return cj
}

func newFileModeJob(job Job, path string) Job {
	oldName := job.par2Name

	job.par2Name = filepath.Base(path) + schema.Par2Extension
	if strings.HasPrefix(oldName, ".") {
		job.par2Name = "." + job.par2Name
	}

	job.par2Path = filepath.Join(job.workingDir, job.par2Name)
	job.manifestName = job.par2Name + schema.ManifestExtension
	job.manifestPath = job.par2Path + schema.ManifestExtension
	job.lockPath = job.par2Path + schema.LockExtension

	return job
}

func (prog *Service) Create(ctx context.Context, rootDir string, opts Options) error {
	errs := []error{}

	logger := prog.creationLogger(ctx, nil, rootDir)
	logger.Info("Scanning filesystem for jobs...")

	jobs, err := prog.Enumerate(ctx, rootDir, opts)
	if err != nil {
		if !errors.Is(err, schema.ErrNonFatal) {
			return fmt.Errorf("failed to enumerate jobs: %w", err)
		}

		err = fmt.Errorf("failed to enumerate some jobs: %w", err)
		errs = append(errs, fmt.Errorf("%w: %w", schema.ErrExitPartialFailure, err))
	}
	prog.considerRecursive(ctx, jobs)

	results := util.NewResultTracker(logger)
	defer results.PrintCompletionInfo(len(jobs))

	if len(jobs) > 0 {
		logger.Info(fmt.Sprintf("Starting to process %d jobs...", len(jobs)),
			"maxDuration", opts.MaxDuration.Value.String())
	} else {
		logger.Info("Nothing to do (will check again next run)")
	}

	var deadlineCtx context.Context //nolint:contextcheck
	var deadlineCancel context.CancelFunc
	if opts.MaxDuration.Value > 0 {
		deadlineCtx, deadlineCancel = context.WithDeadline(ctx, time.Now().Add(opts.MaxDuration.Value))
		defer deadlineCancel()
	}

	for i, job := range jobs {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context error: %w", err)
		}

		if deadlineCtx != nil {
			if err := deadlineCtx.Err(); errors.Is(err, context.DeadlineExceeded) {
				logger := prog.creationLogger(ctx, nil, nil)
				logger.Warn("Exceeded the --duration budget (will continue next run)",
					"unprocessedJobs", len(jobs)-i, "totalJobs", len(jobs),
					"maxDuration", opts.MaxDuration.Value.String())

				break
			}
		}

		pos := fmt.Sprintf("%d/%d", i+1, len(jobs))
		ctx := context.WithValue(ctx, schema.PosKey, pos)

		logger := prog.creationLogger(ctx, job, nil)
		logger.Info("Job started")

		if err := prog.createPar2(ctx, job); err == nil {
			logger.Info("Job completed with success")
			results.Success++
		} else if errors.Is(err, schema.ErrFileIsLocked) {
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

func (prog *Service) Enumerate(ctx context.Context, rootDir string, opts Options) ([]*Job, error) {
	jobs := []*Job{}
	chkr := util.NewIgnoreChecker(prog.fsys)

	var parseErrors int
	err := prog.walker.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context error: %w", err)
		}
		if err != nil {
			logger := prog.creationLogger(ctx, nil, path)
			logger.Warn("A path was skipped due to FS error (will retry next run)", "error", err)

			return nil
		}
		if skip, err := chkr.ShouldSkip(path, d.IsDir()); skip {
			logger := prog.creationLogger(ctx, nil, path)
			logger.Debug("A path was skipped due to a present ignore-file", "error", err)

			return err //nolint:wrapcheck
		}

		if !strings.HasPrefix(filepath.Base(path), createMarkerPathPrefix) {
			return nil
		}

		cfg, err := prog.parseMarkerFile(path, opts)
		if err != nil {
			logger := prog.creationLogger(ctx, nil, path)
			logger.Error("A found marker file could not be parsed (will retry next run)", "error", err)
			parseErrors++

			return nil
		}

		jobs = append(jobs, NewJob(path, *cfg))

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk FS: %w", err)
	}
	if parseErrors > 0 {
		return jobs, fmt.Errorf("%w: %d marker files failed to parse", schema.ErrNonFatal, parseErrors)
	}

	return jobs, nil
}

func (prog *Service) createPar2(ctx context.Context, job *Job) error {
	files, err := prog.findElementsToProtect(ctx, job)
	if err != nil {
		return fmt.Errorf("failed to find to-protect elements: %w", err)
	}

	if job.par2Mode == schema.CreateFileMode {
		if err := prog.createFileMode(ctx, job, files); err != nil {
			return fmt.Errorf("failed to create par2: %w", err)
		}
	} else {
		if err := prog.createFolderMode(ctx, job, files); err != nil {
			return fmt.Errorf("failed to create par2: %w", err)
		}
	}

	if err := prog.fsys.Remove(job.markerPath); err != nil {
		logger := prog.creationLogger(ctx, job, job.markerPath)
		logger.Error("Failed to delete marker file (needs manual deletion)", "error", err)

		return fmt.Errorf("failed to delete marker file: %w", err)
	}

	return nil
}

func (prog *Service) findElementsToProtect(ctx context.Context, job *Job) ([]schema.FsElement, error) {
	dirPaths, err := afero.Glob(prog.fsys, filepath.Join(job.workingDir, job.par2Glob))
	if err != nil {
		logger := prog.creationLogger(ctx, job, job.workingDir)
		logger.Error("Failed to glob folder (will retry next run)", "error", err)

		return nil, fmt.Errorf("failed to glob: %w", err)
	}

	protectableElements := []schema.FsElement{}
	for _, f := range dirPaths {
		if f == job.markerPath {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f), schema.Par2Extension) {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f), schema.Par2Extension+schema.LockExtension) {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f), schema.Par2Extension+schema.ManifestExtension) {
			continue
		}

		fi, err := prog.fsys.Stat(f)
		if err != nil {
			logger := prog.creationLogger(ctx, job, f)
			logger.Error("Failed to stat file (will retry next run)", "error", err)

			return nil, fmt.Errorf("failed to stat: %w", err)
		}

		if fi.IsDir() {
			continue
		}

		protectableElements = append(protectableElements, schema.FsElement{
			Path:    f,
			Name:    fi.Name(),
			Size:    fi.Size(),
			Mode:    fi.Mode(),
			ModTime: fi.ModTime(),
		})
	}

	if len(protectableElements) == 0 {
		logger := prog.creationLogger(ctx, job, job.workingDir)
		logger.Error("No files to protect in folder (will check again next run)")

		return nil, errNoFilesToProtect
	}

	return protectableElements, nil
}

func (prog *Service) createFolderMode(ctx context.Context, job *Job, elements []schema.FsElement) error {
	logger := prog.creationLogger(ctx, job, job.par2Path)

	if _, err := prog.fsys.Stat(job.par2Path); err == nil {
		logger.Warn("Same-named PAR2 already exists in folder (not overwriting)")

		return schema.ErrAlreadyExists
	}

	if err := prog.runCreate(ctx, job, elements); err != nil {
		return err
	}

	logger.Info("Succeeded to create PAR2")

	return nil
}

func (prog *Service) createFileMode(ctx context.Context, job *Job, elements []schema.FsElement) error {
	var errCount int

	for i, f := range elements {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context error: %w", err)
		}

		mpos := fmt.Sprintf("%d/%d", i+1, len(elements))
		ctx := context.WithValue(ctx, schema.MposKey, mpos)

		j := newFileModeJob(*job, f.Path)
		je := []schema.FsElement{elements[i]}
		logger := prog.creationLogger(ctx, &j, j.par2Path)

		if _, err := prog.fsys.Stat(j.par2Path); err == nil {
			logger.Warn("Same-named PAR2 already exists in folder (not overwriting)")

			continue
		}

		if err := prog.runCreate(ctx, &j, je); err != nil {
			errCount++

			continue
		}

		logger.Info("Succeeded to create PAR2")
	}

	if errCount > 0 {
		return fmt.Errorf("%w: %d/%d failed", errSubjobFailure, errCount, len(elements))
	}

	return nil
}

func (prog *Service) runCreate(ctx context.Context, job *Job, elements []schema.FsElement) error {
	logger := prog.creationLogger(ctx, job, job.par2Path)

	var needsCleanup bool
	defer func() {
		if needsCleanup {
			prog.cleanupAfterFailure(ctx, job)
		}
	}()

	unlock, err := util.AcquireLock(prog.fsys, job.lockPath, false)
	if err != nil {
		return fmt.Errorf("failed to lock: %w", err)
	}
	defer unlock()

	cmdArgs := make([]string, 0, 1+len(job.par2Args)+1+1+len(elements))
	cmdArgs = append(cmdArgs, "create")
	cmdArgs = append(cmdArgs, job.par2Args...)
	cmdArgs = append(cmdArgs, "--")
	cmdArgs = append(cmdArgs, job.par2Path)
	cmdArgs = append(cmdArgs, getPaths(elements)...)

	mf := schema.NewManifest(job.par2Name)
	mf.Creation = &schema.CreationManifest{}
	mf.Creation.Args = slices.Clone(job.par2Args)
	mf.Creation.Elements = elements

	mf.Creation.Time = time.Now()
	err = prog.runner.Run(ctx, "par2", cmdArgs, job.workingDir, prog.log.Options.Stdout, prog.log.Options.Stdout)
	mf.Creation.Duration = time.Since(mf.Creation.Time)

	if err != nil {
		needsCleanup = true
		err = fmt.Errorf("par2cmdline: %w", err)

		c := util.AsExitCode(err)
		if c != nil {
			err = fmt.Errorf("%w (%d)", err, *c)
		}

		logger.Error("Failed to create PAR2", "error", err)

		return err
	}

	util.Par2IndexToManifest(prog.fsys, util.Par2IndexToManifestOptions{
		Time:     mf.Creation.Time,
		Path:     job.par2Path,
		Manifest: mf,
	}, logger)

	if sha256hash, err := util.HashFile(prog.fsys, job.par2Path); err != nil {
		logger.Warn("Failed to hash PAR2 for par2cron manifest (will retry on verify)", "error", err)
	} else {
		mf.SHA256 = sha256hash
		if err := util.WriteManifest(prog.fsys, job.manifestPath, mf); err != nil {
			logger := prog.creationLogger(ctx, job, job.manifestPath)
			logger.Warn("Failed to write par2cron manifest (will retry on verify)", "error", err)
		}
	}

	if job.par2Verify {
		vs := verify.NewService(prog.fsys, prog.log, prog.runner)
		vj := verify.NewJob(job.par2Path, verify.Options{}, mf)

		if err := vs.RunVerify(ctx, vj, true); err != nil {
			needsCleanup = true

			return fmt.Errorf("failed to verify par2: %w", err)
		}
	}

	return nil
}

func (prog *Service) cleanupAfterFailure(ctx context.Context, job *Job) {
	basePath := strings.TrimSuffix(job.par2Path, schema.Par2Extension)

	p2f, err := afero.Glob(prog.fsys, basePath+"*"+schema.Par2Extension)
	if err != nil {
		logger := prog.creationLogger(ctx, job, job.workingDir)
		logger.Warn(fmt.Sprintf("Failed to cleanup after failure (%s need manual deletion)", schema.Par2Extension), "error", err)

		return
	}

	p2f = append(p2f, job.manifestPath)
	p2f = append(p2f, job.lockPath)

	for _, f := range p2f {
		// Just to be safe, in case of pathing insanity.
		if !strings.HasSuffix(strings.ToLower(f), schema.Par2Extension) &&
			!strings.HasSuffix(strings.ToLower(f), schema.Par2Extension+schema.ManifestExtension) &&
			!strings.HasSuffix(strings.ToLower(f), schema.Par2Extension+schema.LockExtension) {
			continue
		}

		if err := prog.fsys.Remove(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
			logger := prog.creationLogger(ctx, job, f)
			logger.Warn("Failed to cleanup a file after failure (needs manual deletion)", "error", err)
		}
	}
}

func (prog *Service) considerRecursive(ctx context.Context, jobs []*Job) {
	for _, job := range jobs {
		for _, arg := range job.par2Args {
			if arg == "-R" || arg == "--recursive" {
				logger := prog.creationLogger(ctx, job, nil)
				logger.Warn("par2 argument -R has no effect; " +
					"par2cron creates flat (non-recursive) PAR2 sets by design (see documentation)")

				return
			}
		}
	}
}

func getPaths(files []schema.FsElement) []string {
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}

	return paths
}
