package info

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"time"

	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/desertwitch/par2cron/internal/verify"
)

// Result contains the complete info command output.
type Result struct {
	// Root directories for this result.
	Roots []string `json:"roots"`

	// Time is when this result was generated.
	Time time.Time `json:"time"`

	// Options are the arguments used for this info run.
	Options *Options `json:"options"`

	// Summary contains job counts and duration statistics.
	Summary *Summary `json:"summary"`

	// AgeInfo contains calculations based on the --age constraint.
	AgeInfo *AgeInfo `json:"age_info,omitempty"`

	// DurationInfo contains calculations based on the --duration constraint.
	DurationInfo *DurationInfo `json:"duration_info,omitempty"`

	// BacklogInfo contains backlog health analysis using both --age and --duration.
	BacklogInfo *BacklogInfo `json:"backlog_info,omitempty"`

	// CycleInfo contains verification progress within the current cycle window.
	CycleInfo *CycleInfo `json:"cycle_info,omitempty"`

	// OverdueInfo contains jobs that were due but not verified in time.
	OverdueInfo *OverdueInfo `json:"overdue_info,omitempty"`

	// Warning indicates issues encountered during enumeration.
	Warning string `json:"warning,omitempty"`

	// Prometheus-only values (not part of the JSON output).
	cachedSets      int
	incompleteRoots int
	largestDuration time.Duration
}

// Summary contains aggregate statistics for all discovered jobs.
type Summary struct {
	// JobCount is the total number of jobs found.
	JobCount int `json:"job_count"`

	// KnownCount is the number of jobs with known verification duration.
	KnownCount int `json:"known_count"`

	// UnknownCount is the number of jobs with unknown verification duration.
	UnknownCount int `json:"unknown_count"`

	// Healthies is the number of jobs that passed verification.
	Healthies int `json:"healthies"`

	// Repairables is the number of jobs with repairable corruption.
	Repairables int `json:"repairables"`

	// Unrepairables is the number of jobs with unrepairable corruption.
	Unrepairables int `json:"unrepairables"`

	// Unverifieds is the number of jobs not yet verified.
	Unverifieds int `json:"unverifieds"`

	// AvgDuration is the average verification duration across known jobs.
	AvgDuration time.Duration `json:"avg_duration_ns"`

	// TotalDuration is the sum of all known verification durations.
	TotalDuration time.Duration `json:"total_duration_ns"`

	// FirstVerification is the timestamp of the oldest verification.
	FirstVerification *time.Time `json:"first_verification,omitempty"`

	// LastVerification is the timestamp of the most recent verification.
	LastVerification *time.Time `json:"last_verification,omitempty"`

	// Warning indicates issues with the summary data.
	Warning string `json:"warning,omitempty"`
}

// AgeInfo contains calculations when using --age (disregarding --duration).
type AgeInfo struct {
	// RunsPerCycle is how many runs fit within the --age window.
	RunsPerCycle int `json:"runs_per_cycle"`

	// MinDuration is the floor for --duration with this --age window.
	MinDuration time.Duration `json:"min_duration_ns"`

	// Warning indicates configuration issues with the --age constraint.
	Warning string `json:"warning,omitempty"`
}

// DurationInfo contains calculations when using --duration (disregarding --age).
type DurationInfo struct {
	// RunsNeeded is how many runs are required to verify all jobs.
	RunsNeeded int `json:"runs_needed"`

	// FullCycleEvery is the time to complete a full verification cycle.
	FullCycleEvery time.Duration `json:"full_cycle_every_ns,omitempty"`

	// CompleteInOneRun is true if all jobs can be verified in a single run.
	CompleteInOneRun bool `json:"complete_in_one_run"`

	// Warning indicates configuration issues with the --duration constraint.
	Warning string `json:"warning,omitempty"`

	// LargestJob is the filename of the largest job that exceeds --duration.
	LargestJob string `json:"largest_job,omitempty"`
}

// BacklogInfo contains backlog health when using both --age and --duration.
type BacklogInfo struct {
	// Capacity is the total processing time available per cycle.
	Capacity time.Duration `json:"capacity_ns"`

	// MinRequired is the minimum processing time needed to avoid backlog growth.
	MinRequired time.Duration `json:"min_required_ns"`

	// Margin is the difference between capacity and required (positive is healthy).
	Margin time.Duration `json:"margin_ns"`

	// Healthy is true if capacity exceeds or equals the required time.
	Healthy bool `json:"healthy"`

	// UnknownCount is the number of unknown duration jobs excluded from analysis.
	UnknownCount int `json:"unknown_count,omitempty"`

	// Warning indicates backlog health issues.
	Warning string `json:"warning,omitempty"`
}

// CycleInfo contains verification progress analysis for the rolling --age window.
type CycleInfo struct {
	// VerifiedCount is the number of jobs verified within the --age window.
	VerifiedCount int `json:"verified_count"`

	// TotalCount is the total number of jobs.
	TotalCount int `json:"total_count"`

	// VerifiedPct is the percentage of total jobs verified within the --age window.
	VerifiedPct float64 `json:"verified_pct"`

	// VerifiedDuration is the sum of durations for jobs verified within the --age window.
	VerifiedDuration time.Duration `json:"verified_duration_ns"`

	// TotalDuration is the sum of all known job durations.
	TotalDuration time.Duration `json:"total_duration_ns"`

	// DurationCoveredPct is the percentage of total known duration verified within the --age window.
	DurationCoveredPct float64 `json:"duration_covered_pct"`

	// UnknownCount is the number of unknown duration jobs excluded from analysis.
	UnknownCount int `json:"unknown_count,omitempty"`

	// Warning indicates issues with cycle progress data.
	Warning string `json:"warning,omitempty"`
}

// OverdueInfo contains jobs that were due for verification but not picked up in time.
type OverdueInfo struct {
	// OverdueRunCount is the number of jobs due for longer than one run interval.
	OverdueRunCount int `json:"overdue_run_count"`

	// OverdueCycleCount is the number of jobs due for longer than a full --age cycle.
	OverdueCycleCount int `json:"overdue_cycle_count"`

	// MostOverdueJob is the filename of the job that has been due the longest.
	MostOverdueJob string `json:"most_overdue_job,omitempty"`

	// MostOverdueBy is how long the most overdue job has been due.
	MostOverdueBy time.Duration `json:"most_overdue_by_ns,omitempty"`

	// Warning indicates jobs that have been due for longer than one full cycle.
	Warning string `json:"warning,omitempty"`
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
	if opts.RunInterval.Value <= 0 {
		return nil, fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, errNoCalcInterval)
	}

	now := time.Now()

	vs := verify.NewService(prog.fsys, prog.logbase, prog.runner, prog.bundler, prog.cacher)
	va := verify.Options{IncludeExternal: opts.IncludeExternal, SkipNotCreated: opts.SkipNotCreated}

	result := &Result{
		Roots:   slices.Clone(rootDirs),
		Time:    now.UTC(),
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
		result.cachedSets += cache.SavedLen() // for Prometheus
		// We don't save the cache so there cannot be races with verification.
		// An info could finish after an overlapping verification and discard
		// the verification progress in a race, so we only let verification
		// write to the cache (seeing info does not mutate manifests anyway).

		metas = append(metas, meta...)
	}
	if err := errors.Join(errs...); err != nil {
		result.Warning = fmt.Sprintf("Not all manifests could be read: %v", err)
	}
	result.incompleteRoots = len(errs) // for Prometheus

	js := vs.Stats(metas)
	result.Summary = &Summary{
		JobCount:      js.JobCount,
		KnownCount:    js.KnownCount,
		UnknownCount:  js.UnknownCount,
		Healthies:     js.Healthies,
		Repairables:   js.Repairables,
		Unrepairables: js.Unrepairables,
		Unverifieds:   js.Unverifieds,
		TotalDuration: js.TotalDuration,
		AvgDuration:   js.AvgDuration,
	}
	result.largestDuration = js.LargestDuration // for Prometheus

	if !js.FirstVerification.IsZero() {
		result.Summary.FirstVerification = &js.FirstVerification
	}
	if !js.LastVerification.IsZero() {
		result.Summary.LastVerification = &js.LastVerification
	}

	if js.KnownCount == 0 {
		result.Summary.Warning = "No duration data available, run a full verification to establish baseline"

		return result, nil
	}

	if opts.MinAge.Value > 0 {
		result.AgeInfo = prog.buildAgeInfo(js, opts)
	}

	if opts.MaxDuration.Value > 0 {
		result.DurationInfo = prog.buildDurationInfo(js, opts)
	}

	if opts.MinAge.Value > 0 && opts.MaxDuration.Value > 0 {
		result.BacklogInfo = prog.buildBacklogInfo(js, opts)
	}

	if opts.MinAge.Value > 0 && js.TotalDuration > 0 && js.JobCount > 0 {
		result.CycleInfo = prog.buildCycleInfo(js, metas, opts, now)
	}

	if opts.MinAge.Value > 0 {
		result.OverdueInfo = prog.buildOverdueInfo(metas, opts, now)
	}

	return result, nil
}

func (prog *Service) buildAgeInfo(js verify.Stats, opts Options) *AgeInfo {
	runsPerCycle := max(int(opts.MinAge.Value/opts.RunInterval.Value), 1)
	requiredDuration := max(js.TotalDuration/time.Duration(runsPerCycle), time.Second)

	info := &AgeInfo{
		RunsPerCycle: runsPerCycle,
		MinDuration:  requiredDuration,
	}

	if opts.MinAge.Value < opts.RunInterval.Value {
		info.Warning = "min_age is less than run_interval; files will always be stale"
	}

	return info
}

func (prog *Service) buildDurationInfo(js verify.Stats, opts Options) *DurationInfo {
	runsNeeded := max(int((js.TotalDuration+opts.MaxDuration.Value-1)/opts.MaxDuration.Value), 1)
	cycleLength := time.Duration(runsNeeded) * opts.RunInterval.Value
	singleRun := js.TotalDuration <= opts.MaxDuration.Value

	info := &DurationInfo{
		RunsNeeded:       runsNeeded,
		CompleteInOneRun: singleRun,
	}

	if !singleRun {
		info.FullCycleEvery = cycleLength
	}

	if js.LargestDuration > opts.MaxDuration.Value {
		info.Warning = fmt.Sprintf("Largest job (%s) exceeds max_duration; will overshoot soft limit", util.FmtDur(js.LargestDuration))
		info.LargestJob = filepath.Base(js.LargestJob.Par2Path)
	}

	return info
}

func (prog *Service) buildBacklogInfo(js verify.Stats, opts Options) *BacklogInfo {
	runsPerCycle := max(int(opts.MinAge.Value/opts.RunInterval.Value), 1)
	capacity := time.Duration(runsPerCycle) * opts.MaxDuration.Value
	margin := capacity - js.TotalDuration

	info := &BacklogInfo{
		Capacity:    capacity,
		MinRequired: js.TotalDuration,
		Margin:      margin,
		Healthy:     margin >= 0,
	}

	if js.UnknownCount > 0 {
		info.UnknownCount = js.UnknownCount
	}

	if margin < 0 {
		info.Warning = "Backlog is unhealthy; will grow indefinitely with current arguments"
	}

	return info
}

func (prog *Service) buildCycleInfo(js verify.Stats, jobs []*verify.JobMeta, opts Options, now time.Time) *CycleInfo {
	cycleStart := now.Add(-opts.MinAge.Value)

	var verifiedCount int
	var verifiedDuration time.Duration
	for _, job := range jobs {
		if job.HasVerification {
			if job.VerifyTime.After(cycleStart) {
				verifiedCount++
				verifiedDuration += job.VerifyDuration
			}
		}
	}

	countPct := float64(verifiedCount) / float64(js.JobCount) * 100            //nolint:mnd
	durationPct := float64(verifiedDuration) / float64(js.TotalDuration) * 100 //nolint:mnd

	info := &CycleInfo{
		VerifiedCount:      verifiedCount,
		TotalCount:         js.JobCount,
		VerifiedPct:        countPct,
		VerifiedDuration:   verifiedDuration,
		TotalDuration:      js.TotalDuration,
		DurationCoveredPct: durationPct,
	}

	if js.UnknownCount > 0 {
		info.UnknownCount = js.UnknownCount
		info.Warning = fmt.Sprintf("cycle_info excludes %d unknown duration jobs", info.UnknownCount)
	}

	return info
}

func (prog *Service) buildOverdueInfo(jobs []*verify.JobMeta, opts Options, now time.Time) *OverdueInfo {
	info := &OverdueInfo{}

	var maxLagJob *verify.JobMeta
	for _, job := range jobs {
		if !job.HasVerification || job.VerifyTime.IsZero() {
			continue
		}

		// How long the job has been due (negative = not yet due).
		lag := now.Sub(job.VerifyTime.Add(opts.MinAge.Value))
		if lag <= opts.RunInterval.Value {
			continue // Due jobs are expected to be picked up within one run.
		}

		info.OverdueRunCount++
		if lag > opts.MinAge.Value {
			info.OverdueCycleCount++
		}

		if lag > info.MostOverdueBy {
			info.MostOverdueBy = lag
			maxLagJob = job
		}
	}

	if maxLagJob != nil {
		info.MostOverdueJob = filepath.Base(maxLagJob.Par2Path)
	}

	if info.OverdueCycleCount > 0 {
		info.Warning = fmt.Sprintf("%d jobs have been due for longer than one full cycle; "+
			"check for backlog warnings, repeated failures, and if par2cron actually runs", info.OverdueCycleCount)
	}

	return info
}
