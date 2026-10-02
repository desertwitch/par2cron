package info

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/desertwitch/par2cron/internal/schema"
)

var errPrometheusWithJSON = errors.New("--prometheus cannot be combined with --json")

// promWriter renders gauge families in the Prometheus text format (0.0.4).
// Every line is terminated with "\n" (as required by the Pushgateway), and
// samples of a family must be written directly after its family() call.
type promWriter struct {
	strings.Builder
}

var (
	promHelpEscaper  = strings.NewReplacer(`\`, `\\`, "\n", `\n`)
	promLabelEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
)

func (w *promWriter) family(name, help string) {
	_, _ = w.WriteString("# HELP ")
	_, _ = w.WriteString(name)
	_, _ = w.WriteString(" ")
	_, _ = w.WriteString(promHelpEscaper.Replace(help))
	_, _ = w.WriteString("\n")

	_, _ = w.WriteString("# TYPE ")
	_, _ = w.WriteString(name)
	_, _ = w.WriteString(" gauge")
	_, _ = w.WriteString("\n")
}

// sample writes one sample; labelPairs alternates label names and values.
func (w *promWriter) sample(name string, value float64, labelPairs ...string) {
	if len(labelPairs)%2 != 0 {
		panic("promWriter.sample: odd number of label arguments for " + name)
	}

	_, _ = w.WriteString(name)

	if len(labelPairs) > 0 {
		_, _ = w.WriteString("{")
		for i := 0; i+1 < len(labelPairs); i += 2 {
			if i > 0 {
				_, _ = w.WriteString(",")
			}
			_, _ = w.WriteString(labelPairs[i])
			_, _ = w.WriteString(`=`)
			_, _ = w.WriteString(`"`)
			_, _ = w.WriteString(promLabelEscaper.Replace(labelPairs[i+1]))
			_, _ = w.WriteString(`"`)
		}
		_, _ = w.WriteString("}")
	}

	_, _ = w.WriteString(" ")
	_, _ = w.WriteString(strconv.FormatFloat(value, 'g', -1, 64))
	_, _ = w.WriteString("\n")
}

func (w *promWriter) gauge(name, help string, value float64) {
	w.family(name, help)
	w.sample(name, value)
}

func unixSeconds(t time.Time) float64 {
	return float64(t.Unix())
}

// exportPrometheus writes the info result to standard output in the
// Prometheus text exposition format (0.0.4), for use with the node_exporter
// textfile collector or a Pushgateway (PUT).
func (prog *Service) exportPrometheus(ctx context.Context, rootDirs []string, opts Options) error {
	if prog.log.Options.WantJSON {
		return fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, errPrometheusWithJSON)
	}

	start := time.Now()

	result, err := prog.Result(ctx, rootDirs, opts)
	if err != nil {
		return fmt.Errorf("failed to get result: %w", err)
	}

	scanDuration := time.Since(start)

	prog.logResultWarnings(result)

	version, goVersion := buildVersions()
	out := renderPrometheus(result, scanDuration, version, goVersion)

	if _, err := io.WriteString(prog.log.Options.Stdout, out); err != nil {
		return fmt.Errorf("failed to write metrics: %w", err)
	}

	return nil
}

// logResultWarnings sends all human-readable warnings of the result to the
// structured log (standard error), since standard output carries the metrics.
func (prog *Service) logResultWarnings(r *Result) {
	debugs := []string{}
	warnings := []string{r.Warning}

	if r.Summary != nil {
		warnings = append(warnings, r.Summary.Warning)
	}
	if r.AgeInfo != nil {
		warnings = append(warnings, r.AgeInfo.Warning)
	}
	if r.DurationInfo != nil {
		warnings = append(warnings, r.DurationInfo.Warning)
	}
	if r.BacklogInfo != nil {
		warnings = append(warnings, r.BacklogInfo.Warning)
	}
	if r.CycleInfo != nil {
		// CycleInfo.Warning is informational only (unknown durations are normal).
		debugs = append(debugs, r.CycleInfo.Warning)
	}
	if r.OverdueInfo != nil {
		warnings = append(warnings, r.OverdueInfo.Warning)
	}

	for _, d := range debugs {
		if d != "" {
			prog.log.Debug(d)
		}
	}

	for _, w := range warnings {
		if w != "" {
			prog.log.Warn(w)
		}
	}
}

// buildVersions returns the par2cron version and Go version for the
// build_info metric. The version is the one injected via -ldflags into
// schema.ProgramVersion (same as reported elsewhere by the program).
func buildVersions() (string, string) {
	version := schema.ProgramVersion
	if version == "" {
		version = "unknown"
	}

	return version, runtime.Version()
}

// renderPrometheus renders the result as Prometheus text exposition format.
// It is a pure function (no clock, no I/O) so that it can be golden-tested.
//
// Presence of metrics:
//
//   - Always:
//     --> build_info
//     --> scan_*
//     --> sets
//     --> sets_duration_unknown
//     --> verify_duration_known_seconds
//     --> verify_duration_largest_seconds
//     --> config_run_interval_seconds
//
//   - verify_*_timestamp_seconds:
//     --> at least one set has been verified.
//
//   - config_age_seconds:
//     --> Argument --age given
//
//   - config_duration_seconds:
//     --> Argument --duration given
//
//   - sets_verified_within_age, verify_duration_within_age_seconds:
//     --> Argument --age given and known durations exist (CycleInfo).
//
//   - sets_overdue, sets_overdue_cycle, sets_most_overdue_seconds:
//     --> Argument --age given and known durations exist (OverdueInfo).
//
//   - backlog_margin_seconds:
//     --> Arguments --age and --duration given and known durations exist (BacklogInfo).
//
//nolint:funlen
func renderPrometheus(r *Result, scanDuration time.Duration, version, goVersion string) string {
	var w promWriter
	s := r.Summary

	// par2cron_build_info
	w.family(
		"par2cron_build_info",
		"A metric with a constant '1' value labeled by version and goversion from which par2cron was built.",
	)
	w.sample(
		"par2cron_build_info", 1,
		"goversion", goVersion,
		"version", version,
	)

	// par2cron_scan_duration_seconds
	w.gauge(
		"par2cron_scan_duration_seconds",
		"Time taken to scan the given directories for PAR2 sets and load their manifests.",
		scanDuration.Seconds(),
	)

	// par2cron_scan_incomplete_roots
	w.gauge(
		"par2cron_scan_incomplete_roots",
		"Number of given root directories in which not all par2cron manifests could be read; "+
			"if non-zero, the other metric values may be incomplete.",
		float64(r.incompleteRoots),
	)

	// par2cron_sets
	w.family(
		"par2cron_sets",
		"Number of PAR2 sets by status of their last verification, where unverified means never verified; "+
			"the statuses sum to the total number of sets.",
	)
	w.sample(
		"par2cron_sets", float64(s.Healthies),
		"status", "healthy",
	)
	w.sample(
		"par2cron_sets", float64(s.Repairables),
		"status", "repairable",
	)
	w.sample(
		"par2cron_sets", float64(s.Unrepairables),
		"status", "unrepairable",
	)
	w.sample(
		"par2cron_sets", float64(s.Unverifieds),
		"status", "unverified",
	)

	// par2cron_sets_duration_unknown
	w.gauge(
		"par2cron_sets_duration_unknown",
		"Number of PAR2 sets without a recorded verification duration (typically never verified); "+
			"these are excluded from all duration-based values.",
		float64(s.UnknownCount),
	)

	// par2cron_verify_duration_known_seconds
	w.gauge(
		"par2cron_verify_duration_known_seconds",
		"Sum of the last recorded verification durations of all PAR2 sets with a known duration, "+
			"i.e. an estimate of the time one full verification pass takes, excluding sets with unknown duration.",
		s.TotalDuration.Seconds(),
	)

	// par2cron_verify_duration_largest_seconds
	w.gauge(
		"par2cron_verify_duration_largest_seconds",
		"Longest last recorded verification duration of a single PAR2 set; "+
			"a set longer than --duration overshoots the soft limit whenever it is verified.",
		r.largestDuration.Seconds(),
	)

	// par2cron_verify_oldest_timestamp_seconds
	if s.FirstVerification != nil {
		w.gauge(
			"par2cron_verify_oldest_timestamp_seconds",
			"Unix time of the least recent last verification across all verified PAR2 sets; "+
				"every verified set was verified at or after this time. Absent if no set has been verified.",
			unixSeconds(*s.FirstVerification),
		)
	}

	// par2cron_verify_newest_timestamp_seconds
	if s.LastVerification != nil {
		w.gauge(
			"par2cron_verify_newest_timestamp_seconds",
			"Unix time of the most recent last verification across all verified PAR2 sets; "+
				"every verified set was verified before or at this time. Absent if no set has been verified.",
			unixSeconds(*s.LastVerification),
		)
	}

	// par2cron_config_run_interval_seconds
	w.gauge(
		"par2cron_config_run_interval_seconds",
		"How often par2cron verify is assumed to run, as given to info with --calc-run-interval (default 24h).",
		r.Options.RunInterval.Value.Seconds(),
	)

	// par2cron_config_age_seconds
	if r.Options.MinAge.Value > 0 {
		w.gauge(
			"par2cron_config_age_seconds",
			"Target time between re-verifications of a PAR2 set, as given to info with --age; "+
				"only meaningful if it matches the --age used by par2cron verify.",
			r.Options.MinAge.Value.Seconds(),
		)
	}

	// par2cron_config_duration_seconds
	if r.Options.MaxDuration.Value > 0 {
		w.gauge(
			"par2cron_config_duration_seconds",
			"Time budget per verify run, as given to info with --duration; "+
				"only meaningful if it matches the --duration used by par2cron verify.",
			r.Options.MaxDuration.Value.Seconds(),
		)
	}

	// par2cron_sets_verified_within_age
	// par2cron_verify_duration_within_age_seconds
	if c := r.CycleInfo; c != nil {
		w.gauge(
			"par2cron_sets_verified_within_age",
			"Number of PAR2 sets last verified within the past --age, i.e. within the current rolling cycle window.",
			float64(c.VerifiedCount),
		)

		w.gauge(
			"par2cron_verify_duration_within_age_seconds",
			"Sum of the last recorded verification durations of PAR2 sets verified within the past --age; "+
				"divide by par2cron_verify_duration_known_seconds for duration-weighted cycle progress.",
			c.VerifiedDuration.Seconds(),
		)
	}

	// par2cron_sets_overdue
	// par2cron_sets_overdue_cycle
	// par2cron_sets_most_overdue_seconds
	if o := r.OverdueInfo; o != nil {
		w.gauge(
			"par2cron_sets_overdue",
			"Number of verified PAR2 sets whose re-verification (last verification plus --age) has been due "+
				"for longer than one run interval; includes the sets counted in par2cron_sets_overdue_cycle.",
			float64(o.OverdueRunCount),
		)

		w.gauge(
			"par2cron_sets_overdue_cycle",
			"Number of verified PAR2 sets whose re-verification has been due for longer than one full --age cycle; "+
				"non-zero suggests a growing backlog, repeated failures or par2cron verify not running.",
			float64(o.OverdueCycleCount),
		)

		w.gauge(
			"par2cron_sets_most_overdue_seconds",
			"Longest time a set counted in par2cron_sets_overdue has been due for re-verification; 0 if none.",
			o.MostOverdueBy.Seconds(),
		)
	}

	// par2cron_backlog_margin_seconds
	if b := r.BacklogInfo; b != nil {
		w.gauge(
			"par2cron_backlog_margin_seconds",
			"Verification capacity per --age cycle (runs per cycle, i.e. --age divided by the run interval "+
				"rounded down and at least 1, times --duration) minus par2cron_verify_duration_known_seconds; "+
				"negative means the backlog grows indefinitely. Sets with unknown duration are not included.",
			b.Margin.Seconds(),
		)
	}

	return w.String()
}
