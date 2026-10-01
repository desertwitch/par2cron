package list

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"time"

	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/verify"
)

// Result contains the complete list command output.
type Result struct {
	// Root directories for this result.
	Roots []string `json:"roots"`

	// Time is when this result was generated.
	Time time.Time `json:"time"`

	// Options are the arguments used for this list run.
	Options *Options `json:"options"`

	// Jobs are the discovered jobs, ordered by severity and then path.
	Jobs []*Job `json:"jobs"`

	// Warning indicates issues encountered during enumeration.
	Warning string `json:"warning,omitempty"`
}

// Job describes a single discovered job.
type Job struct {
	// Path is the full path to the job's PAR2 file.
	Path string `json:"path"`

	// Status is the derived job status (e.g. healthy, repairable).
	Status string `json:"status"`

	// IsBundle is true if the job is a bundle.
	IsBundle bool `json:"is_bundle"`

	// IsCached is true if the job exists in provided --cache.
	IsCached bool `json:"is_cached"`

	// HasManifest is true if the job has a manifest.
	HasManifest bool `json:"has_manifest"`

	// HasCreation is true if the manifest contains creation data.
	HasCreation bool `json:"has_creation"`

	// HasVerification is true if the manifest contains verification data.
	HasVerification bool `json:"has_verification"`

	// Verification contains the last verification result, if any.
	Verification *Verification `json:"verification,omitempty"`
}

// Verification contains the result of a job's last verification.
type Verification struct {
	// Time is when the last verification happened.
	Time time.Time `json:"time"`

	// Duration is how long the last verification took.
	Duration time.Duration `json:"duration_ns"`

	// CountCorrupted is the number of corrupted blocks found.
	CountCorrupted int `json:"count_corrupted"`

	// RepairNeeded is true if corruption was found.
	RepairNeeded bool `json:"repair_needed"`

	// RepairPossible is true if the corruption can be repaired.
	RepairPossible bool `json:"repair_possible"`

	// MaybeEdited is true if the files may have been modified since creation.
	MaybeEdited bool `json:"maybe_edited"`
}

func toJob(m *verify.JobMeta) *Job {
	job := &Job{
		Path:            m.Par2Path,
		Status:          statusOf(m).String(),
		IsBundle:        m.IsBundle,
		IsCached:        m.Saved,
		HasManifest:     m.HasManifest,
		HasCreation:     m.HasCreation,
		HasVerification: m.HasVerification,
	}

	if m.HasVerification {
		job.Verification = &Verification{
			Time:           m.VerifyTime,
			Duration:       m.VerifyDuration,
			CountCorrupted: m.CountCorrupted,
			RepairNeeded:   m.RepairNeeded,
			RepairPossible: m.RepairPossible,
			MaybeEdited:    m.MaybeEdited,
		}
	}

	return job
}

func (prog *Service) PrintJSON(ctx context.Context, rootDirs []string, opts Options) error {
	result, err := prog.Result(ctx, rootDirs, opts)
	if err != nil {
		return fmt.Errorf("failed to get result: %w", err)
	}

	enc := json.NewEncoder(prog.log.Options.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return fmt.Errorf("failed to encode result: %w", err)
	}

	return nil
}

func (prog *Service) openCacheJSON(rootDir string, opts Options, result *Result) schema.Cache {
	cache := prog.cacher.NewCache(prog.fsys, opts.CacheDir, rootDir)

	if opts.CacheDir == "" {
		return cache
	}

	if err := cache.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		result.Warning = fmt.Sprintf("Manifest cache for '%s' could not be loaded: %v", rootDir, err)
	}

	return cache
}

func (prog *Service) Result(ctx context.Context, rootDirs []string, opts Options) (*Result, error) {
	vs := verify.NewService(prog.fsys, prog.logbase, prog.runner, prog.bundler, prog.cacher)
	va := verify.Options{IncludeExternal: false, SkipNotCreated: opts.SkipNotCreated}

	result := &Result{
		Roots:   slices.Clone(rootDirs),
		Time:    time.Now().UTC(),
		Options: &opts,
	}

	metas := []*verify.JobMeta{}
	errs := []error{}
	for _, rootDir := range rootDirs {
		cache := prog.openCacheJSON(rootDir, opts, result)

		meta, err := vs.Enumerate(ctx, rootDir, va, cache)
		if err != nil {
			if !errors.Is(err, schema.ErrNonFatal) {
				return nil, fmt.Errorf("%s: failed to enumerate jobs: %w", rootDir, err)
			}

			errs = append(errs, fmt.Errorf("%s: %w", rootDir, err))
		}

		cache.PruneUnwalked()
		// We don't save the cache so there cannot be races with verification.
		// A list could finish after an overlapping verification and discard
		// the verification progress in a race, so we only let verification
		// write to the cache (seeing a list does not mutate manifests anyway).

		metas = append(metas, meta...)
	}
	if err := errors.Join(errs...); err != nil {
		result.Warning = fmt.Sprintf("Not all manifests could be read: %v", err)
	}

	slices.SortFunc(metas, func(a, b *verify.JobMeta) int {
		return cmp.Or(
			cmp.Compare(statusOf(a), statusOf(b)),
			strings.Compare(a.Par2Path, b.Par2Path),
		)
	})

	result.Jobs = make([]*Job, 0, len(metas))
	for _, m := range metas {
		result.Jobs = append(result.Jobs, toJob(m))
	}

	return result, nil
}
