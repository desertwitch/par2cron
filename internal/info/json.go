package info

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/desertwitch/par2cron/internal/verify"
)

// Result contains the complete info command output.
type Result struct {
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

	// Warning indicates issues encountered during enumeration.
	Warning string `json:"warning,omitempty"`
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

func (prog *Service) PrintJSON(ctx context.Context, rootDir string, args Options) error {
	result, err := prog.Result(ctx, rootDir, args)
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

func (prog *Service) Result(ctx context.Context, rootDir string, args Options) (*Result, error) {
	if args.RunInterval.Value <= 0 {
		return nil, fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, errNoCalcInterval)
	}

	now := time.Now()

	vs := verify.NewService(prog.fsys, prog.log, prog.runner)
	va := verify.Options{IncludeExternal: args.IncludeExternal, SkipNotCreated: args.SkipNotCreated}

	result := &Result{
		Time:    now,
		Options: &args,
	}

	jobs, err := vs.Enumerate(ctx, rootDir, va)
	if err != nil {
		if !errors.Is(err, schema.ErrNonFatal) {
			return nil, fmt.Errorf("failed to enumerate jobs: %w", err)
		}

		result.Warning = fmt.Sprintf("Not all manifests could be read: %v", err)
	}

	js := vs.Stats(jobs)
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

	if !js.LastVerification.IsZero() {
		result.Summary.LastVerification = &js.LastVerification
	}

	if js.KnownCount == 0 {
		result.Summary.Warning = "No duration data available, run a full verification to establish baseline"

		return result, nil
	}

	if args.MinAge.Value > 0 {
		result.AgeInfo = prog.buildAgeInfo(js, args)
	}

	if args.MaxDuration.Value > 0 {
		result.DurationInfo = prog.buildDurationInfo(js, args)
	}

	if args.MinAge.Value > 0 && args.MaxDuration.Value > 0 {
		result.BacklogInfo = prog.buildBacklogInfo(js, args)
	}

	if args.MinAge.Value > 0 && js.TotalDuration > 0 && js.JobCount > 0 {
		result.CycleInfo = prog.buildCycleInfo(js, jobs, args, now)
	}

	return result, nil
}

func (prog *Service) buildAgeInfo(js verify.Stats, args Options) *AgeInfo {
	runsPerCycle := max(int(args.MinAge.Value/args.RunInterval.Value), 1)
	requiredDuration := max(js.TotalDuration/time.Duration(runsPerCycle), time.Second)

	info := &AgeInfo{
		RunsPerCycle: runsPerCycle,
		MinDuration:  requiredDuration,
	}

	if args.MinAge.Value < args.RunInterval.Value {
		info.Warning = "min_age is less than run_interval; files will always be stale"
	}

	return info
}

func (prog *Service) buildDurationInfo(js verify.Stats, args Options) *DurationInfo {
	runsNeeded := max(int((js.TotalDuration+args.MaxDuration.Value-1)/args.MaxDuration.Value), 1)
	cycleLength := time.Duration(runsNeeded) * args.RunInterval.Value
	singleRun := js.TotalDuration <= args.MaxDuration.Value

	info := &DurationInfo{
		RunsNeeded:       runsNeeded,
		CompleteInOneRun: singleRun,
	}

	if !singleRun {
		info.FullCycleEvery = cycleLength
	}

	if js.LargestDuration > args.MaxDuration.Value {
		info.Warning = fmt.Sprintf("Largest job (%s) exceeds max_duration; will overshoot soft limit", util.FmtDur(js.LargestDuration))
		info.LargestJob = filepath.Base(js.LargestJob.Par2Path())
	}

	return info
}

func (prog *Service) buildBacklogInfo(js verify.Stats, args Options) *BacklogInfo {
	runsPerCycle := max(int(args.MinAge.Value/args.RunInterval.Value), 1)
	capacity := time.Duration(runsPerCycle) * args.MaxDuration.Value
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

func (prog *Service) buildCycleInfo(js verify.Stats, jobs []*verify.Job, args Options, now time.Time) *CycleInfo {
	cycleStart := now.Add(-args.MinAge.Value)

	var verifiedCount int
	var verifiedDuration time.Duration
	for _, job := range jobs {
		if job.Manifest() != nil && job.Manifest().Verification != nil {
			if job.Manifest().Verification.Time.After(cycleStart) {
				verifiedCount++
				verifiedDuration += job.Manifest().Verification.Duration
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
