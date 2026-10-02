package info

import (
	"context"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// hasPromFamily reports whether the output declares the given gauge family.
func hasPromFamily(out string, name string) bool {
	return strings.Contains(out, "# TYPE "+name+" gauge\n")
}

// Expectation: family should write the HELP and TYPE lines for a gauge.
func Test_promWriter_family_Success(t *testing.T) {
	t.Parallel()

	var w promWriter
	w.family("par2cron_test", "A test metric.")

	require.Equal(t, "# HELP par2cron_test A test metric.\n# TYPE par2cron_test gauge\n", w.String())
}

// Expectation: family should escape backslashes and newlines in the HELP text.
func Test_promWriter_family_Escaping_Success(t *testing.T) {
	t.Parallel()

	var w promWriter
	w.family("par2cron_test", "back\\slash and\nnewline")

	require.Equal(t, "# HELP par2cron_test back\\\\slash and\\nnewline\n# TYPE par2cron_test gauge\n", w.String())
}

// Expectation: sample should write a sample without a label set when no labels are given.
func Test_promWriter_sample_NoLabels_Success(t *testing.T) {
	t.Parallel()

	var w promWriter
	w.sample("par2cron_test", 42)

	require.Equal(t, "par2cron_test 42\n", w.String())
}

// Expectation: sample should write all label pairs in the given order, separated by commas.
func Test_promWriter_sample_MultipleLabels_Success(t *testing.T) {
	t.Parallel()

	var w promWriter
	w.sample("par2cron_test", 1.5, "a", "1", "b", "2")

	require.Equal(t, "par2cron_test{a=\"1\",b=\"2\"} 1.5\n", w.String())
}

// Expectation: sample should escape quotes, backslashes and newlines in label values.
func Test_promWriter_sample_Escaping_Success(t *testing.T) {
	t.Parallel()

	var w promWriter
	w.sample("par2cron_test", 1, "label", "quote\" back\\slash\nnewline")

	require.Equal(t, "par2cron_test{label=\"quote\\\" back\\\\slash\\nnewline\"} 1\n", w.String())
}

// Expectation: sample should format values in their shortest exact representation.
func Test_promWriter_sample_ValueFormatting_Success(t *testing.T) {
	t.Parallel()

	var w promWriter
	w.sample("par2cron_a", 0)
	w.sample("par2cron_b", 86400)
	w.sample("par2cron_c", -3000)
	w.sample("par2cron_d", 0.5)
	w.sample("par2cron_e", 1768478400)

	require.Equal(t,
		"par2cron_a 0\n"+
			"par2cron_b 86400\n"+
			"par2cron_c -3000\n"+
			"par2cron_d 0.5\n"+
			"par2cron_e 1.7684784e+09\n",
		w.String())
}

// Expectation: sample should panic when given an odd number of label arguments.
func Test_promWriter_sample_OddLabels_Panics(t *testing.T) {
	t.Parallel()

	var w promWriter

	require.PanicsWithValue(t, "promWriter.sample: odd number of label arguments for par2cron_test", func() {
		w.sample("par2cron_test", 1, "label") //nolint:staticcheck
	})
}

// Expectation: gauge should write a complete family with a single unlabeled sample.
func Test_promWriter_gauge_Success(t *testing.T) {
	t.Parallel()

	var w promWriter
	w.gauge("par2cron_test", "A test metric.", 7)

	require.Equal(t,
		"# HELP par2cron_test A test metric.\n"+
			"# TYPE par2cron_test gauge\n"+
			"par2cron_test 7\n",
		w.String())
}

// Expectation: unixSeconds should return whole Unix seconds, independent of time zone.
func Test_unixSeconds_Success(t *testing.T) {
	t.Parallel()

	utc := time.Date(2026, 1, 15, 12, 0, 0, 999*int(time.Millisecond), time.UTC)
	other := utc.In(time.FixedZone("UTC+2", 2*60*60))

	require.InDelta(t, float64(1768478400), unixSeconds(utc), 0)
	require.InDelta(t, float64(1768478400), unixSeconds(other), 0)
}

// Expectation: exportPrometheus should reject --json as a bad invocation without writing to standard output.
func Test_Service_exportPrometheus_WithJSON_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout:   &logBuf,
		Stdout:   &stdoutBuf,
		Stderr:   io.Discard,
		WantJSON: true,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{Prometheus: true}
	_ = args.RunInterval.Set("24h")
	err := prog.exportPrometheus(t.Context(), []string{"/data"}, args)

	require.ErrorIs(t, err, schema.ErrExitBadInvocation)
	require.ErrorIs(t, err, errPrometheusWithJSON)
	require.Empty(t, stdoutBuf.Bytes())
}

// Expectation: exportPrometheus should return an error without writing to standard output when no run interval is given.
func Test_Service_exportPrometheus_NoRunInterval_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{Prometheus: true}
	err := prog.exportPrometheus(t.Context(), []string{"/data"}, args)

	require.ErrorIs(t, err, schema.ErrExitBadInvocation)
	require.ErrorIs(t, err, errNoCalcInterval)
	require.Empty(t, stdoutBuf.Bytes())
}

// Expectation: A cancellation should be respected, the correct error returned and nothing written to standard output.
func Test_Service_exportPrometheus_CtxCancel_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{Prometheus: true}
	_ = args.RunInterval.Set("24h")
	err := prog.exportPrometheus(ctx, []string{"/data"}, args)

	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, stdoutBuf.Bytes())
}

// Expectation: exportPrometheus should write valid metrics for the found jobs, including all --age and --duration metrics.
func Test_Service_exportPrometheus_WithJobs_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     time.Now(),
		Duration: 5 * time.Minute,
	}
	require.NoError(t, writeTestManifest(t, fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, manifest))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{Prometheus: true}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("1h")
	require.NoError(t, prog.exportPrometheus(t.Context(), []string{"/data"}, args))

	out := string(stdoutBuf.Bytes())
	checkExposition(t, out)

	require.Contains(t, out, "par2cron_scan_incomplete_roots 0\n")
	require.Contains(t, out, "par2cron_verify_duration_known_seconds 300\n")
	require.Contains(t, out, "par2cron_verify_duration_largest_seconds 300\n")
	require.Contains(t, out, "par2cron_config_run_interval_seconds 86400\n")
	require.Contains(t, out, "par2cron_config_age_seconds 604800\n")
	require.Contains(t, out, "par2cron_config_duration_seconds 3600\n")
	require.Contains(t, out, "par2cron_sets_verified_within_age 1\n")
	require.Contains(t, out, "par2cron_verify_duration_within_age_seconds 300\n")
	require.Contains(t, out, "par2cron_sets_overdue 0\n")
	require.Contains(t, out, "par2cron_sets_overdue_cycle 0\n")
	require.Contains(t, out, "par2cron_sets_most_overdue_seconds 0\n")
	require.Contains(t, out, "par2cron_backlog_margin_seconds 24900\n") // 7 runs * 1h - 5m

	require.True(t, hasPromFamily(out, "par2cron_verify_oldest_timestamp_seconds"))
	require.True(t, hasPromFamily(out, "par2cron_verify_newest_timestamp_seconds"))
}

// Expectation: exportPrometheus should omit the duration-based metrics and log a warning when no duration data exists.
func Test_Service_exportPrometheus_NoKnownDurations_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{Prometheus: true}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("1h")
	require.NoError(t, prog.exportPrometheus(t.Context(), []string{"/data"}, args))

	out := string(stdoutBuf.Bytes())
	checkExposition(t, out)

	require.True(t, hasPromFamily(out, "par2cron_config_age_seconds"))
	require.True(t, hasPromFamily(out, "par2cron_config_duration_seconds"))

	require.False(t, hasPromFamily(out, "par2cron_sets_verified_within_age"))
	require.False(t, hasPromFamily(out, "par2cron_sets_overdue"))
	require.False(t, hasPromFamily(out, "par2cron_backlog_margin_seconds"))

	require.Contains(t, string(logBuf.Bytes()), "No duration data available")
	require.NotContains(t, out, "No duration data available")
}

// Expectation: exportPrometheus should report an unhealthy backlog as a negative margin and log its warning.
func Test_Service_exportPrometheus_UnhealthyBacklog_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     time.Now(),
		Duration: 2 * time.Hour,
	}
	require.NoError(t, writeTestManifest(t, fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, manifest))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{Prometheus: true}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	_ = args.MaxDuration.Set("10m")
	require.NoError(t, prog.exportPrometheus(t.Context(), []string{"/data"}, args))

	out := string(stdoutBuf.Bytes())
	checkExposition(t, out)

	require.Contains(t, out, "par2cron_backlog_margin_seconds -3000\n") // 7 runs * 10m - 2h
	require.Contains(t, out, "par2cron_verify_duration_largest_seconds 7200\n")

	require.Contains(t, string(logBuf.Bytes()), "Backlog is unhealthy")
	require.NotContains(t, out, "Backlog is unhealthy")
}

// Expectation: exportPrometheus should report an overdue job in the nested overdue metrics.
func Test_Service_exportPrometheus_OverdueCycle_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/test"+schema.Par2Extension, []byte("par2"), 0o644))

	manifest := schema.NewManifest("test" + schema.Par2Extension)
	manifest.Verification = &schema.VerificationManifest{
		Time:     time.Now().Add(-20 * 24 * time.Hour),
		Duration: 5 * time.Minute,
	}
	require.NoError(t, writeTestManifest(t, fs, "/data/test"+schema.Par2Extension+schema.ManifestExtension, manifest))

	var stdoutBuf testutil.SafeBuffer
	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: &stdoutBuf,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	args := Options{Prometheus: true}
	_ = args.RunInterval.Set("24h")
	_ = args.MinAge.Set("7d")
	require.NoError(t, prog.exportPrometheus(t.Context(), []string{"/data"}, args))

	out := string(stdoutBuf.Bytes())
	checkExposition(t, out)

	require.Contains(t, out, "par2cron_sets_overdue 1\n")
	require.Contains(t, out, "par2cron_sets_overdue_cycle 1\n")
	require.Contains(t, out, "par2cron_sets_verified_within_age 0\n")
	require.NotContains(t, out, "par2cron_sets_most_overdue_seconds 0\n")

	require.Contains(t, string(logBuf.Bytes()), "due for longer than one full cycle")
}

// Expectation: logResultWarnings should log every non-empty warning of the result,
// except the informational cycle warning, which is only logged at debug level.
func Test_Service_logResultWarnings_AllWarnings_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	result := &Result{
		Warning:      "warning-result",
		Summary:      &Summary{Warning: "warning-summary"},
		AgeInfo:      &AgeInfo{Warning: "warning-age"},
		DurationInfo: &DurationInfo{Warning: "warning-duration"},
		BacklogInfo:  &BacklogInfo{Warning: "warning-backlog"},
		CycleInfo:    &CycleInfo{Warning: "warning-cycle"},
		OverdueInfo:  &OverdueInfo{Warning: "warning-overdue"},
	}

	prog.logResultWarnings(result)

	logs := string(logBuf.Bytes())
	for _, w := range []string{
		"warning-result",
		"warning-summary",
		"warning-age",
		"warning-duration",
		"warning-backlog",
		"warning-overdue",
	} {
		require.Contains(t, logs, w)
	}

	require.NotContains(t, logs, "warning-cycle")
}

// Expectation: logResultWarnings should log the informational cycle warning at debug level.
func Test_Service_logResultWarnings_CycleWarningDebug_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	result := &Result{
		Summary:   &Summary{},
		CycleInfo: &CycleInfo{Warning: "warning-cycle"},
	}

	prog.logResultWarnings(result)

	require.Contains(t, string(logBuf.Bytes()), "warning-cycle")
}

// Expectation: logResultWarnings should not log anything when there are no warnings.
func Test_Service_logResultWarnings_NoWarnings_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &testutil.MockCacheHandler{})

	result := &Result{
		Summary:      &Summary{},
		AgeInfo:      &AgeInfo{},
		DurationInfo: &DurationInfo{},
		BacklogInfo:  &BacklogInfo{},
		CycleInfo:    &CycleInfo{},
		OverdueInfo:  &OverdueInfo{},
	}

	prog.logResultWarnings(result)

	require.Empty(t, logBuf.Bytes())
}

// Expectation: buildVersions should return the injected program version (or "unknown") and the Go version.
func Test_buildVersions_Success(t *testing.T) {
	t.Parallel()

	version, goVersion := buildVersions()

	require.Equal(t, runtime.Version(), goVersion)

	if schema.ProgramVersion == "" {
		require.Equal(t, "unknown", version)
	} else {
		require.Equal(t, schema.ProgramVersion, version)
	}
}

// Expectation: renderPrometheus should always emit the unconditional metrics and omit all conditional ones.
func Test_renderPrometheus_Minimal_Success(t *testing.T) {
	t.Parallel()

	args := Options{}
	_ = args.RunInterval.Set("24h")

	result := &Result{
		Options: &args,
		Summary: &Summary{JobCount: 3, UnknownCount: 1, Healthies: 1, Repairables: 2},
	}

	out := renderPrometheus(result, time.Second, "1.2.3", "go1.26.0")
	checkExposition(t, out)

	for _, name := range []string{
		"par2cron_build_info",
		"par2cron_scan_duration_seconds",
		"par2cron_scan_incomplete_roots",
		"par2cron_sets",
		"par2cron_sets_duration_unknown",
		"par2cron_verify_duration_known_seconds",
		"par2cron_verify_duration_largest_seconds",
		"par2cron_config_run_interval_seconds",
	} {
		require.True(t, hasPromFamily(out, name), "missing always-present family %s", name)
	}

	for _, name := range []string{
		"par2cron_verify_oldest_timestamp_seconds",
		"par2cron_verify_newest_timestamp_seconds",
		"par2cron_config_age_seconds",
		"par2cron_config_duration_seconds",
		"par2cron_sets_verified_within_age",
		"par2cron_verify_duration_within_age_seconds",
		"par2cron_sets_overdue",
		"par2cron_sets_overdue_cycle",
		"par2cron_sets_most_overdue_seconds",
		"par2cron_backlog_margin_seconds",
	} {
		require.False(t, hasPromFamily(out, name), "unexpected conditional family %s", name)
	}

	require.Contains(t, out, "par2cron_build_info{goversion=\"go1.26.0\",version=\"1.2.3\"} 1\n")
	require.Contains(t, out, "par2cron_sets{status=\"healthy\"} 1\n")
	require.Contains(t, out, "par2cron_sets{status=\"repairable\"} 2\n")
	require.Contains(t, out, "par2cron_sets{status=\"unrepairable\"} 0\n")
	require.Contains(t, out, "par2cron_sets{status=\"unverified\"} 0\n")
	require.Contains(t, out, "par2cron_sets_duration_unknown 1\n")
	require.Contains(t, out, "par2cron_config_run_interval_seconds 86400\n")
}

// Expectation: renderPrometheus should emit the verification timestamps as Unix seconds when present.
func Test_renderPrometheus_Timestamps_Success(t *testing.T) {
	t.Parallel()

	args := Options{}
	_ = args.RunInterval.Set("24h")

	first := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	last := first.Add(time.Hour)

	result := &Result{
		Options: &args,
		Summary: &Summary{FirstVerification: &first, LastVerification: &last},
	}

	out := renderPrometheus(result, time.Second, "1.2.3", "go1.26.0")
	checkExposition(t, out)

	require.Contains(t, out, "par2cron_verify_oldest_timestamp_seconds 1.7684784e+09\n")
	require.Contains(t, out, "par2cron_verify_newest_timestamp_seconds 1.768482e+09\n")
}
