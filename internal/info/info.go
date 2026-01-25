package info

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/desertwitch/par2cron/internal/flags"
	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/desertwitch/par2cron/internal/verify"
	"github.com/spf13/afero"
)

var errNoCalcInterval = errors.New("no run interval provided")

type Options struct {
	MinAge          flags.Duration `json:"min_age"`
	MaxDuration     flags.Duration `json:"max_duration"`
	RunInterval     flags.Duration `json:"run_interval"`
	IncludeExternal bool           `json:"include_external"`
	SkipNotCreated  bool           `json:"skip_not_created"`
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

func (prog *Service) Info(ctx context.Context, rootDir string, args Options) error {
	if prog.log.Options.WantJSON {
		return prog.PrintJSON(ctx, rootDir, args)
	}

	if args.RunInterval.Value <= 0 {
		fmt.Fprintf(prog.log.Options.Stdout, "You need to define how often you run par2cron with --calc-run-interval\n")
		fmt.Fprintf(prog.log.Options.Stdout, "\n")

		return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, errNoCalcInterval)
	}

	fmt.Fprintln(prog.log.Options.Stdout, "Scanning filesystem for jobs...")
	fmt.Fprintf(prog.log.Options.Stdout, "\n")

	now := time.Now()

	vs := verify.NewService(prog.fsys, prog.log, prog.runner)
	va := verify.Options{IncludeExternal: args.IncludeExternal, SkipNotCreated: args.SkipNotCreated}

	jobs, err := vs.Enumerate(ctx, rootDir, va)
	if err != nil {
		if !errors.Is(err, schema.ErrNonFatal) {
			return fmt.Errorf("failed to enumerate jobs: %w", err)
		}

		fmt.Fprintf(prog.log.Options.Stdout, "Warning: Not all manifests could be read (%v)\n", err)
		fmt.Fprintf(prog.log.Options.Stdout, "\n")
	}

	js := vs.Stats(jobs)

	fmt.Fprintf(prog.log.Options.Stdout, "Total jobs found: %d (%d with known duration, %d with unknown duration)\n",
		js.JobCount, js.KnownCount, js.UnknownCount)
	fmt.Fprintf(prog.log.Options.Stdout, "Total jobs status: %d healthy, %d repairable, %d unrepairable, %d unverified\n",
		js.Healthies, js.Repairables, js.Unrepairables, js.Unverifieds)
	fmt.Fprintf(prog.log.Options.Stdout, "Total verification time: %s\n", util.FmtDur(js.TotalDuration))
	fmt.Fprintf(prog.log.Options.Stdout, "Average job duration: %s\n", util.FmtDur(js.AvgDuration))
	if !js.LastVerification.IsZero() {
		fmt.Fprintf(prog.log.Options.Stdout, "Last verification time: %s\n", js.LastVerification.Format(time.RFC1123))
	}
	fmt.Fprintf(prog.log.Options.Stdout, "\n")

	if js.KnownCount == 0 {
		fmt.Fprintf(prog.log.Options.Stdout, "Warning: No duration data available, run a full verification to establish baseline\n")
		fmt.Fprintf(prog.log.Options.Stdout, "\n")

		return nil
	}

	if args.MinAge.Value > 0 {
		prog.printAgeInfo(js, args)
	}

	if args.MaxDuration.Value > 0 {
		prog.printDurationInfo(js, args)
	}

	if args.MinAge.Value > 0 && args.MaxDuration.Value > 0 {
		prog.printBacklogInfo(js, args)
	}

	if args.MinAge.Value > 0 && js.TotalDuration > 0 && js.JobCount > 0 {
		prog.printCycleInfo(js, jobs, args, now)
	}

	return nil
}

func (prog *Service) printAgeInfo(js verify.Stats, args Options) {
	if args.MinAge.Value <= 0 {
		return
	}

	runsPerCycle := max(int(args.MinAge.Value/args.RunInterval.Value), 1)
	requiredDuration := max(js.TotalDuration/time.Duration(runsPerCycle), time.Second)

	fmt.Fprintf(prog.log.Options.Stdout, "With just --age %s (no --duration, running par2cron every %s):\n", &args.MinAge, &args.RunInterval)
	fmt.Fprintf(prog.log.Options.Stdout, "  Runs per verification cycle: %d\n", runsPerCycle)
	fmt.Fprintf(prog.log.Options.Stdout, "  If using --duration, minimum should be: %s\n", util.FmtDur(requiredDuration))
	fmt.Fprintf(prog.log.Options.Stdout, "\n")

	if args.MinAge.Value < args.RunInterval.Value {
		fmt.Fprintf(prog.log.Options.Stdout, "Warning: --age (%s) is less than --calc-run-interval (%s)\n", &args.MinAge, &args.RunInterval)
		fmt.Fprintf(prog.log.Options.Stdout, "  Files will always be stale, increase --age or run more frequently\n")
		fmt.Fprintf(prog.log.Options.Stdout, "\n")
	}
}

func (prog *Service) printDurationInfo(js verify.Stats, args Options) {
	if args.MaxDuration.Value <= 0 {
		return
	}

	runsNeeded := max(int((js.TotalDuration+args.MaxDuration.Value-1)/args.MaxDuration.Value), 1)
	cycleLength := time.Duration(runsNeeded) * args.RunInterval.Value

	fmt.Fprintf(prog.log.Options.Stdout, "With just --duration %s (no --age, running par2cron every %s):\n", &args.MaxDuration, &args.RunInterval)
	fmt.Fprintf(prog.log.Options.Stdout, "  Runs needed to achieve a full verification: %d\n", runsNeeded)

	if js.TotalDuration <= args.MaxDuration.Value {
		fmt.Fprintf(prog.log.Options.Stdout, "  A full verification is achieved in a single run\n")
	} else {
		fmt.Fprintf(prog.log.Options.Stdout, "  A full verification is eventually achieved every: %s\n", util.FmtDur(cycleLength))
	}
	fmt.Fprintf(prog.log.Options.Stdout, "\n")

	if js.LargestDuration > args.MaxDuration.Value {
		fmt.Fprintf(prog.log.Options.Stdout, "Warning: Largest job (%s) exceeds --duration %s\n", util.FmtDur(js.LargestDuration), &args.MaxDuration)
		fmt.Fprintf(prog.log.Options.Stdout, "  Job: %s\n", filepath.Base(js.LargestJob.Par2Path()))
		fmt.Fprintf(prog.log.Options.Stdout, "  At least one job will overshoot the soft duration limit when it runs (to avoid starvation)\n")
		fmt.Fprintf(prog.log.Options.Stdout, "\n")
	}
}

func (prog *Service) printBacklogInfo(js verify.Stats, args Options) {
	if args.MinAge.Value <= 0 || args.MaxDuration.Value <= 0 {
		return
	}

	runsPerCycle := max(int(args.MinAge.Value/args.RunInterval.Value), 1)
	capacity := time.Duration(runsPerCycle) * args.MaxDuration.Value
	margin := capacity - js.TotalDuration

	fmt.Fprintf(prog.log.Options.Stdout, "With --age %s and --duration %s (running par2cron every %s):\n", &args.MinAge, &args.MaxDuration, &args.RunInterval)
	fmt.Fprintf(prog.log.Options.Stdout, "  Processing capacity: %s\n", util.FmtDur(capacity))
	fmt.Fprintf(prog.log.Options.Stdout, "  Minimum needed to avoid backlog growing: %s\n", util.FmtDur(js.TotalDuration))

	if margin >= 0 {
		status := "HEALTHY"
		if js.UnknownCount > 0 {
			status = "HEALTHY (based on known durations)"
		}
		fmt.Fprintf(prog.log.Options.Stdout, "  Backlog: %s (margin: %s)\n", status, util.FmtDur(margin))
		fmt.Fprintf(prog.log.Options.Stdout, "\n")
	} else {
		fmt.Fprintf(prog.log.Options.Stdout, "  Backlog: UNHEALTHY (shortfall: %s)\n", util.FmtDur(-margin))
		fmt.Fprintf(prog.log.Options.Stdout, "    To clear: run once without --duration, then fix arguments (increase --age and/or --duration)\n")
		fmt.Fprintf(prog.log.Options.Stdout, "    With the current arguments, the backlog will continue to grow indefinitely (INSANE CONFIGURATION)\n")
		fmt.Fprintf(prog.log.Options.Stdout, "\n")
	}
}

func (prog *Service) printCycleInfo(js verify.Stats, jobs []*verify.Job, args Options, now time.Time) {
	if args.MinAge.Value <= 0 || js.TotalDuration <= 0 || js.JobCount == 0 {
		return
	}

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

	fmt.Fprintf(prog.log.Options.Stdout, "Verification progress (--age %s, rolling window):\n", &args.MinAge)
	fmt.Fprintf(prog.log.Options.Stdout, "  Jobs verified: %d/%d (%.1f%%)\n", verifiedCount, js.JobCount, countPct)
	fmt.Fprintf(prog.log.Options.Stdout, "  Known duration covered:\n")
	fmt.Fprintf(prog.log.Options.Stdout, "    %s/%s (%.1f%%)\n", util.FmtDur(verifiedDuration), util.FmtDur(js.TotalDuration), durationPct)
	if js.UnknownCount > 0 {
		fmt.Fprintf(prog.log.Options.Stdout, "    (which excludes %d jobs with unknown duration)\n", js.UnknownCount)
	}
	fmt.Fprintf(prog.log.Options.Stdout, "\n")
}
