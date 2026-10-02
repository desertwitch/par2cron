package info

//go:generate go test -run Test_renderPrometheus_Golden -update .

import (
	"flag"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/flags"
)

// Fixed inputs, so that the rendered output is fully deterministic.
const (
	goldenVersion   = "1.2.3"
	goldenGoVersion = "go1.26.0"
	goldenScan      = 1500 * time.Millisecond

	day = 24 * time.Hour
)

var (
	goldenNow      = time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	updateGolden   = flag.Bool("update", false, "update golden files in testdata/prometheus")
	promMetricName = regexp.MustCompile(`^par2cron_[a-z0-9_]+$`)
)

func dur(d time.Duration) flags.Duration { return flags.Duration{Value: d} }

// checkExposition verifies the structural rules that the node_exporter
// textfile collector and the Pushgateway rely on, independently of the
// golden files, so that format mistakes fail with a precise message.
//
//nolint:gocognit,cyclop
func checkExposition(t *testing.T, out string) {
	t.Helper()

	if out == "" {
		t.Fatal("output is empty (an empty PUT would wipe a Pushgateway group)")
	}
	if !strings.HasSuffix(out, "\n") {
		t.Fatal("output does not end with a newline")
	}
	if strings.Contains(out, "\r") {
		t.Fatal("output contains a carriage return")
	}

	seen := map[string]bool{}
	samples := map[string]int{}
	current := ""

	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]

		switch {
		case strings.HasPrefix(line, "# HELP "):
			name, help, _ := strings.Cut(strings.TrimPrefix(line, "# HELP "), " ")
			if !promMetricName.MatchString(name) {
				t.Errorf("line %d: invalid or unprefixed metric name %q", i+1, name)
			}
			if strings.HasSuffix(name, "_total") || strings.HasSuffix(name, "_count") ||
				strings.HasSuffix(name, "_sum") || strings.HasSuffix(name, "_bucket") {
				t.Errorf("line %d: gauge %q uses a counter/summary/histogram suffix", i+1, name)
			}
			if help == "" {
				t.Errorf("line %d: empty HELP for %q", i+1, name)
			}
			if seen[name] {
				t.Errorf("line %d: family %q is not contiguous or declared twice", i+1, name)
			}
			if i+1 >= len(lines) || lines[i+1] != "# TYPE "+name+" gauge" {
				t.Errorf("line %d: HELP for %q is not followed by its gauge TYPE line", i+1, name)
			}
			seen[name] = true
			current = name
			i++

		case strings.HasPrefix(line, "#"):
			t.Errorf("line %d: unexpected comment line %q", i+1, line)

		default:
			name := line
			if idx := strings.IndexAny(line, "{ "); idx >= 0 {
				name = line[:idx]
			}
			if name != current {
				t.Errorf("line %d: sample of %q outside of its family (current %q)", i+1, name, current)
			}

			rest := strings.TrimPrefix(line, name)
			if strings.HasPrefix(rest, "{") {
				end := strings.LastIndex(rest, "}")
				if end < 0 {
					t.Errorf("line %d: unterminated label set", i+1)

					continue
				}
				if strings.Contains(rest[:end], "job=") || strings.Contains(rest[:end], "instance=") {
					t.Errorf("line %d: job/instance labels must come from the grouping key", i+1)
				}
				rest = rest[end+1:]
			}

			value := strings.TrimPrefix(rest, " ")
			if strings.Contains(value, " ") {
				t.Errorf("line %d: sample has a timestamp or trailing data: %q", i+1, line)
			}
			f, err := strconv.ParseFloat(value, 64)
			if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
				t.Errorf("line %d: invalid sample value %q", i+1, value)
			}
			samples[name]++
		}
	}

	for name := range seen {
		if samples[name] == 0 {
			t.Errorf("family %q has no samples", name)
		}
	}
}

// goldenCases mirror the presence rules documented on renderPrometheus.
// Values are kept consistent with how Result() would compute them.
func goldenCases() map[string]*Result {
	return map[string]*Result{
		// No sets at all: only the always-present metrics, no timestamps.
		"empty": {
			Options: &Options{RunInterval: dur(day)},
			Summary: &Summary{},
		},

		// Sets exist but none was verified yet (KnownCount == 0): --age is
		// given, but Result() returns early, so no cycle/overdue/backlog.
		"unverified_only": {
			Options: &Options{MinAge: dur(7 * day), MaxDuration: dur(2 * time.Hour), RunInterval: dur(day)},
			Summary: &Summary{JobCount: 3, UnknownCount: 3, Unverifieds: 3},
		},

		// Only --duration given: config_duration, but no --age based metrics.
		"duration_only": {
			Options: &Options{MaxDuration: dur(2 * time.Hour), RunInterval: dur(day)},
			Summary: &Summary{
				JobCount: 4, KnownCount: 4, Healthies: 4,
				TotalDuration:     5 * time.Hour,
				FirstVerification: new(goldenNow.Add(-3 * day)),
				LastVerification:  new(goldenNow.Add(-1 * time.Hour)),
			},
			DurationInfo:    &DurationInfo{RunsNeeded: 3},
			largestDuration: 150 * time.Minute,
		},

		// Only --age given, everything on schedule.
		"age_only": {
			Options: &Options{MinAge: dur(7 * day), RunInterval: dur(day)},
			Summary: &Summary{
				JobCount: 5, KnownCount: 5, Healthies: 5,
				TotalDuration:     10 * time.Hour,
				FirstVerification: new(goldenNow.Add(-6 * day)),
				LastVerification:  new(goldenNow.Add(-2 * time.Hour)),
			},
			AgeInfo:         &AgeInfo{RunsPerCycle: 7},
			CycleInfo:       &CycleInfo{VerifiedCount: 5, TotalCount: 5, VerifiedDuration: 10 * time.Hour},
			OverdueInfo:     &OverdueInfo{},
			largestDuration: 3 * time.Hour,
		},

		// Everything present: corruption, unknown durations, incomplete scan,
		// cache in use, overdue sets (nested counts) and a negative backlog margin.
		// 7 runs per cycle * 2h = 14h capacity - 20h known = -6h margin.
		"full": {
			Options: &Options{
				MinAge: dur(7 * day), MaxDuration: dur(2 * time.Hour), RunInterval: dur(day),
				CacheDir: "/cache",
			},
			Summary: &Summary{
				JobCount: 10, KnownCount: 8, UnknownCount: 2,
				Healthies: 6, Repairables: 1, Unrepairables: 1, Unverifieds: 2,
				TotalDuration:     20 * time.Hour,
				FirstVerification: new(goldenNow.Add(-15 * day)),
				LastVerification:  new(goldenNow.Add(-1 * time.Hour)),
			},
			AgeInfo:         &AgeInfo{RunsPerCycle: 7},
			DurationInfo:    &DurationInfo{RunsNeeded: 10},
			BacklogInfo:     &BacklogInfo{Capacity: 14 * time.Hour, MinRequired: 20 * time.Hour, Margin: -6 * time.Hour},
			CycleInfo:       &CycleInfo{VerifiedCount: 5, TotalCount: 10, VerifiedDuration: 12 * time.Hour},
			OverdueInfo:     &OverdueInfo{OverdueRunCount: 3, OverdueCycleCount: 1, MostOverdueBy: 8 * day},
			incompleteRoots: 1,
			largestDuration: 3 * time.Hour,
			cachedSets:      8,
		},
	}
}

// Test_renderPrometheus_Golden compares the rendered output byte for byte
// against the checked-in files in testdata/prometheus. The output is a
// public contract: any diff here must be intentional and reviewed.
func Test_renderPrometheus_Golden(t *testing.T) {
	t.Parallel()

	for name, result := range goldenCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := renderPrometheus(result, goldenScan, goldenVersion, goldenGoVersion)

			checkExposition(t, got)

			path := filepath.Join("testdata", "prometheus", name+".prom")

			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil { //nolint:gosec
					t.Fatal(err)
				}

				return
			}

			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("missing golden file (run go generate): %v", err)
			}

			if got != string(want) {
				t.Errorf("output differs from %s (run go generate if intended)\n--- got ---\n%s\n--- want ---\n%s",
					path, got, want)
			}
		})
	}
}
