package verify

import (
	"io"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: The calculations should be performed to the expectations.
func Test_Service_Stats_Success(t *testing.T) {
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

	timeA := time.Now().Add(-5 * time.Minute)
	timeB := time.Now().Add(-10 * time.Minute)

	metas := []*JobMeta{
		{
			&schema.JobMeta{
				HasManifest:     true,
				HasVerification: true,
				VerifyTime:      timeA,
				VerifyDuration:  5 * time.Minute,
				RepairNeeded:    false,
				RepairPossible:  true,
			},
		},
		{
			&schema.JobMeta{
				HasManifest:     true,
				HasVerification: true,
				VerifyTime:      timeB,
				VerifyDuration:  10 * time.Minute,
				RepairNeeded:    true,
				RepairPossible:  true,
			},
		},
		{
			&schema.JobMeta{
				HasManifest:     true,
				HasVerification: true,
				VerifyDuration:  0,
				RepairNeeded:    true,
				RepairPossible:  false,
			},
		},
		{
			&schema.JobMeta{
				HasManifest: false,
			},
		},
	}

	js := prog.Stats(metas)

	require.Equal(t, 4, js.JobCount)
	require.Equal(t, 2, js.KnownCount)
	require.Equal(t, 2, js.UnknownCount)
	require.Equal(t, 1, js.Unverifieds)
	require.Equal(t, 1, js.Healthies)
	require.Equal(t, 1, js.Repairables)
	require.Equal(t, 1, js.Unrepairables)
	require.Equal(t, 15*time.Minute, js.TotalDuration)
	require.Equal(t, 7*time.Minute+30*time.Second, js.AvgDuration)
	require.True(t, js.LastVerification.Equal(timeA))
}

// Expectation: The function should not error on no jobs available.
func Test_Service_Stats_NoJobs_Success(t *testing.T) {
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

	js := prog.Stats([]*JobMeta{})

	require.Zero(t, js.JobCount)
	require.Zero(t, js.KnownCount)
	require.Zero(t, js.UnknownCount)
	require.Zero(t, js.TotalDuration)
	require.Zero(t, js.Unverifieds)
	require.Zero(t, js.Healthies)
	require.Zero(t, js.Repairables)
	require.Zero(t, js.Unrepairables)
}

// Expectation: The calculations should be performed correctly with unknown jobs.
func Test_Service_Stats_WithUnknownDurations_Success(t *testing.T) {
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

	metas := []*JobMeta{
		{
			&schema.JobMeta{},
		},
		{
			&schema.JobMeta{
				HasManifest:     true,
				HasVerification: true,
				VerifyDuration:  5 * time.Minute,
			},
		},
	}

	js := prog.Stats(metas)

	require.Equal(t, 2, js.JobCount)
	require.Equal(t, 1, js.KnownCount)
	require.Equal(t, 1, js.UnknownCount)
	require.Equal(t, 5*time.Minute, js.TotalDuration)
}

// Expectation: The largest job duration should be identified correctly.
func Test_Service_Stats_IdentifyLargestDuration_Success(t *testing.T) {
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

	meta1 := &JobMeta{
		&schema.JobMeta{
			Par2Path:        "/data/job1" + schema.Par2Extension,
			HasManifest:     true,
			HasVerification: true,
			VerifyDuration:  5 * time.Minute,
		},
	}
	meta2 := &JobMeta{
		&schema.JobMeta{
			Par2Path:        "/data/job2" + schema.Par2Extension,
			HasManifest:     true,
			HasVerification: true,
			VerifyDuration:  20 * time.Minute,
		},
	}
	metas := []*JobMeta{meta1, meta2}

	js := prog.Stats(metas)

	require.Equal(t, 20*time.Minute, js.LargestDuration)
	require.Equal(t, meta2.JobMeta, js.LargestJob)
}

// Expecation: The correct priority should be returned.
func Test_Job_queuePriority_NoManifest_Success(t *testing.T) {
	t.Parallel()

	meta := &JobMeta{&schema.JobMeta{}}
	priority := meta.queuePriority()

	require.Equal(t, prioNoManifest, priority)
}

// Expecation: The correct priority should be returned.
func Test_Job_queuePriority_NoVerification_Success(t *testing.T) {
	t.Parallel()

	meta := &JobMeta{
		&schema.JobMeta{
			HasManifest: true,
		},
	}
	priority := meta.queuePriority()

	require.Equal(t, prioNoVerification, priority)
}

// Expecation: The correct priority should be returned.
func Test_Job_queuePriority_NeedsRepair_Success(t *testing.T) {
	t.Parallel()

	meta := &JobMeta{
		&schema.JobMeta{
			HasManifest:     true,
			HasVerification: true,
			RepairNeeded:    true,
		},
	}
	priority := meta.queuePriority()

	require.Equal(t, prioNeedsRepair, priority)
}

// Expecation: The correct priority should be returned.
func Test_Job_queuePriority_Normal_Success(t *testing.T) {
	t.Parallel()

	meta := &JobMeta{
		&schema.JobMeta{
			HasManifest:     true,
			HasVerification: true,
			RepairNeeded:    false,
		},
	}
	priority := meta.queuePriority()

	require.Equal(t, prioOther, priority)
}

// Expectation: A zero time should be returned when no manifest exists.
func Test_Job_lastVerified_NoManifest_Success(t *testing.T) {
	t.Parallel()

	meta := &JobMeta{&schema.JobMeta{}}
	t0 := meta.lastVerified()

	require.Zero(t, t0)
}

// Expectation: The correct time should be returned when a manifest exists.
func Test_Job_lastVerified_WithVerification_Success(t *testing.T) {
	t.Parallel()

	now := time.Now()
	meta := &JobMeta{
		&schema.JobMeta{
			HasManifest:     true,
			HasVerification: true,
			VerifyTime:      now,
		},
	}
	t0 := meta.lastVerified()

	require.Equal(t, now, t0)
}

// Expectation: A zero duration should be returned when no manifest exists.
func Test_Job_lastDuration_NoManifest_Success(t *testing.T) {
	t.Parallel()

	meta := &JobMeta{&schema.JobMeta{}}
	duration := meta.lastDuration()

	require.Zero(t, duration)
}

// Expectation: The correct duration should be returned when a manifest exists.
func Test_Job_lastDuration_WithVerification_Success(t *testing.T) {
	t.Parallel()

	meta := &JobMeta{
		&schema.JobMeta{
			HasManifest:     true,
			HasVerification: true,
			VerifyDuration:  5 * time.Minute,
		},
	}
	duration := meta.lastDuration()

	require.Equal(t, 5*time.Minute, duration)
}

// Expectation: A question mark should be printed if no manifest exists.
func Test_Job_lastDurationStr_NoManifest_Success(t *testing.T) {
	t.Parallel()

	meta := &JobMeta{&schema.JobMeta{}}
	result := meta.lastDurationStr()

	require.Equal(t, "?", result)
}

// Expectation: A question mark should be printed if no verification exists.
func Test_Job_lastDurationStr_NoVerification_Success(t *testing.T) {
	t.Parallel()

	meta := &JobMeta{
		&schema.JobMeta{
			HasManifest: true,
		},
	}
	result := meta.lastDurationStr()

	require.Equal(t, "?", result)
}

// Expectation: The correct duration string should be returned for the duration.
func Test_Job_lastDurationStr_WithVerification_Success(t *testing.T) {
	t.Parallel()

	meta := &JobMeta{
		&schema.JobMeta{
			HasManifest:     true,
			HasVerification: true,
			VerifyDuration:  5 * time.Minute,
		},
	}
	result := meta.lastDurationStr()

	require.NotEqual(t, "?", result)
	require.Equal(t, "5m0s", result)
}

// Expectation: All jobs should be returned when no --age is given.
func Test_filterByAge_NoMinAge_Success(t *testing.T) {
	t.Parallel()

	metas := []*JobMeta{
		{&schema.JobMeta{Par2Path: "/data/test1" + schema.Par2Extension}},
		{&schema.JobMeta{Par2Path: "/data/test2" + schema.Par2Extension}},
	}
	filtered := filterByAge(metas, 0)

	require.Len(t, filtered, 2)
}

// Expectation: The relevant jobs should be returned when --age is given.
func Test_filterByAge_WithMinAge_Success(t *testing.T) {
	t.Parallel()

	oldTime := time.Now().Add(-48 * time.Hour)
	recentTime := time.Now().Add(-1 * time.Hour)

	metas := []*JobMeta{
		{
			&schema.JobMeta{
				Par2Path:        "/data/old" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyTime:      oldTime,
			},
		},
		{
			&schema.JobMeta{
				Par2Path:        "/data/recent" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyTime:      recentTime,
			},
		},
	}
	filtered := filterByAge(metas, 24*time.Hour)

	require.Len(t, filtered, 1)
	require.Equal(t, "/data/old"+schema.Par2Extension, filtered[0].Par2Path)
}

// Expectation: Jobs without manifest should always be returned.
func Test_filterByAge_NoVerification_Success(t *testing.T) {
	t.Parallel()

	metas := []*JobMeta{
		{
			&schema.JobMeta{
				Par2Path: "/data/test" + schema.Par2Extension,
			},
		},
	}
	filtered := filterByAge(metas, 24*time.Hour)

	require.Len(t, filtered, 1)
}

// Expectation: All jobs should be returned without given --duration.
func Test_filterByDuration_NoMaxDuration_Success(t *testing.T) {
	t.Parallel()

	metas := []*JobMeta{
		{&schema.JobMeta{Par2Path: "/data/test1" + schema.Par2Extension}},
		{&schema.JobMeta{Par2Path: "/data/test2" + schema.Par2Extension}},
	}
	filtered := filterByDuration(metas, 0)

	require.Len(t, filtered, 2)
}

// Expectation: The first job should always be included regardless of duration.
func Test_filterByDuration_FirstJobAlwaysIncluded_Success(t *testing.T) {
	t.Parallel()

	metas := []*JobMeta{
		{
			&schema.JobMeta{
				Par2Path:        "/data/large" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyDuration:  10 * time.Hour,
			},
		},
	}
	filtered := filterByDuration(metas, 1*time.Hour)

	require.Len(t, filtered, 1)
}

// Expectation: The --duration should be respected and not exceeded.
func Test_filterByDuration_FitsMultiple_Success(t *testing.T) {
	t.Parallel()

	metas := []*JobMeta{
		{
			&schema.JobMeta{
				Par2Path:        "/data/job1" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyDuration:  30 * time.Minute,
			},
		},
		{
			&schema.JobMeta{
				Par2Path:        "/data/job2" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyDuration:  20 * time.Minute,
			},
		},
		{
			&schema.JobMeta{
				Par2Path:        "/data/job3" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyDuration:  15 * time.Minute,
			},
		},
	}
	filtered := filterByDuration(metas, 1*time.Hour)

	require.Len(t, filtered, 2)
}

// Expectation: Jobs with unknown duration should always be included regardless of max duration.
func Test_filterByDuration_UnknownDurationAlwaysIncluded_Success(t *testing.T) {
	t.Parallel()

	metas := []*JobMeta{
		{
			&schema.JobMeta{
				Par2Path:        "/data/job1" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyDuration:  30 * time.Minute,
			},
		},
		{
			&schema.JobMeta{
				Par2Path:        "/data/unknown1" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyDuration:  0,
			},
		},
		{
			&schema.JobMeta{
				Par2Path: "/data/unknown2" + schema.Par2Extension,
			},
		},
		{
			&schema.JobMeta{
				Par2Path:        "/data/job2" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyDuration:  45 * time.Minute,
			},
		},
	}
	filtered := filterByDuration(metas, 1*time.Hour)

	require.Len(t, filtered, 3)
	require.Equal(t, "/data/job1"+schema.Par2Extension, filtered[0].Par2Path)
	require.Equal(t, "/data/unknown1"+schema.Par2Extension, filtered[1].Par2Path)
	require.Equal(t, "/data/unknown2"+schema.Par2Extension, filtered[2].Par2Path)
}

// Expectation: Unknown duration jobs should be included even when they would exceed max duration.
func Test_filterByDuration_UnknownDurationExceedingLimit_Success(t *testing.T) {
	t.Parallel()

	metas := []*JobMeta{
		{
			&schema.JobMeta{
				Par2Path:        "/data/job1" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyDuration:  50 * time.Minute,
			},
		},
		{
			&schema.JobMeta{
				Par2Path:        "/data/unknown" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyDuration:  0,
			},
		},
		{
			&schema.JobMeta{
				Par2Path:        "/data/job2" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyDuration:  20 * time.Minute,
			},
		},
	}
	filtered := filterByDuration(metas, 1*time.Hour)

	require.Len(t, filtered, 2)
	require.Equal(t, "/data/job1"+schema.Par2Extension, filtered[0].Par2Path)
	require.Equal(t, "/data/unknown"+schema.Par2Extension, filtered[1].Par2Path)
}

// Expectation: The correct sorting should be done according to priority.
func Test_sortJobs_Success(t *testing.T) {
	t.Parallel()

	metas := []*JobMeta{
		{
			&schema.JobMeta{
				Par2Path:        "/data/normal" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				RepairNeeded:    false,
			},
		},
		{
			&schema.JobMeta{
				Par2Path:        "/data/needs-repair" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				RepairNeeded:    true,
			},
		},
		{
			&schema.JobMeta{
				Par2Path: "/data/no-manifest" + schema.Par2Extension,
			},
		},
	}
	sortJobs(metas)

	require.Equal(t, "/data/no-manifest"+schema.Par2Extension, metas[0].Par2Path)
	require.Equal(t, "/data/needs-repair"+schema.Par2Extension, metas[1].Par2Path)
	require.Equal(t, "/data/normal"+schema.Par2Extension, metas[2].Par2Path)
}

// Expectation: Jobs with the same priority should be sorted by last verified time.
func Test_sortJobs_SamePriority_SortByTime_Success(t *testing.T) {
	t.Parallel()

	oldTime := time.Now().Add(-48 * time.Hour)
	recentTime := time.Now().Add(-24 * time.Hour)

	metas := []*JobMeta{
		{
			&schema.JobMeta{
				Par2Path:        "/data/recent" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyTime:      recentTime,
				RepairNeeded:    false,
			},
		},
		{
			&schema.JobMeta{
				Par2Path:        "/data/old" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyTime:      oldTime,
				RepairNeeded:    false,
			},
		},
	}
	sortJobs(metas)

	require.Equal(t, "/data/old"+schema.Par2Extension, metas[0].Par2Path)
	require.Equal(t, "/data/recent"+schema.Par2Extension, metas[1].Par2Path)
}

// Expectation: Jobs with the same priority and time should be sorted by path.
func Test_sortJobs_SamePriorityAndTime_SortByPath_Success(t *testing.T) {
	t.Parallel()

	sameTime := time.Now()

	metas := []*JobMeta{
		{
			&schema.JobMeta{
				Par2Path:        "/data/zebra" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyTime:      sameTime,
				RepairNeeded:    false,
			},
		},
		{
			&schema.JobMeta{
				Par2Path:        "/data/apple" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyTime:      sameTime,
				RepairNeeded:    false,
			},
		},
	}

	sortJobs(metas)

	require.Equal(t, "/data/apple"+schema.Par2Extension, metas[0].Par2Path)
	require.Equal(t, "/data/zebra"+schema.Par2Extension, metas[1].Par2Path)
}

// Expectation: Complex sorting should respect priority first, then time, then path.
func Test_sortJobs_ComplexSorting_Success(t *testing.T) {
	t.Parallel()

	oldTime := time.Now().Add(-72 * time.Hour)
	midTime := time.Now().Add(-48 * time.Hour)
	recentTime := time.Now().Add(-24 * time.Hour)
	metas := []*JobMeta{
		{
			&schema.JobMeta{
				Par2Path:        "/data/normal-recent" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyTime:      recentTime,
				RepairNeeded:    false,
			},
		},
		{
			&schema.JobMeta{
				Par2Path:        "/data/repair-old" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyTime:      oldTime,
				RepairNeeded:    true,
			},
		},
		{
			&schema.JobMeta{
				Par2Path: "/data/no-manifest" + schema.Par2Extension,
			},
		},
		{
			&schema.JobMeta{
				Par2Path:        "/data/normal-old" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyTime:      oldTime,
				RepairNeeded:    false,
			},
		},
		{
			&schema.JobMeta{
				Par2Path:        "/data/repair-mid" + schema.Par2Extension,
				HasManifest:     true,
				HasVerification: true,
				VerifyTime:      midTime,
				RepairNeeded:    true,
			},
		},
	}
	sortJobs(metas)

	// Priority order: no manifest, needs repair (by time), normal (by time)
	require.Equal(t, "/data/no-manifest"+schema.Par2Extension, metas[0].Par2Path)
	require.Equal(t, "/data/repair-old"+schema.Par2Extension, metas[1].Par2Path)
	require.Equal(t, "/data/repair-mid"+schema.Par2Extension, metas[2].Par2Path)
	require.Equal(t, "/data/normal-old"+schema.Par2Extension, metas[3].Par2Path)
	require.Equal(t, "/data/normal-recent"+schema.Par2Extension, metas[4].Par2Path)
}
