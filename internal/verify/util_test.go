package verify

import (
	"io"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	timeA := time.Now().Add(-5 * time.Minute)
	timeB := time.Now().Add(-10 * time.Minute)

	jobs := []*Job{
		{
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Time:           timeA,
					Duration:       5 * time.Minute,
					RepairNeeded:   false,
					RepairPossible: true,
				},
			},
		},
		{
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Time:           timeB,
					Duration:       10 * time.Minute,
					RepairNeeded:   true,
					RepairPossible: true,
				},
			},
		},
		{
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration:       0,
					RepairNeeded:   true,
					RepairPossible: false,
				},
			},
		},
		{
			manifest: nil,
		},
	}

	js := prog.Stats(jobs)

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	js := prog.Stats([]*Job{})

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	jobs := []*Job{
		{manifest: nil},
		{
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration: 5 * time.Minute,
				},
			},
		},
	}

	js := prog.Stats(jobs)

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

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	job1 := &Job{
		par2Path: "/data/job1" + schema.Par2Extension,
		manifest: &schema.Manifest{
			Verification: &schema.VerificationManifest{
				Duration: 5 * time.Minute,
			},
		},
	}

	job2 := &Job{
		par2Path: "/data/job2" + schema.Par2Extension,
		manifest: &schema.Manifest{
			Verification: &schema.VerificationManifest{
				Duration: 20 * time.Minute,
			},
		},
	}

	jobs := []*Job{job1, job2}

	js := prog.Stats(jobs)

	require.Equal(t, 20*time.Minute, js.LargestDuration)
	require.Equal(t, job2, js.LargestJob)
}

// Expectation: The manifest should be returned correctly when it exists.
func Test_Job_Manifest_WithManifest_Success(t *testing.T) {
	t.Parallel()

	manifest := &schema.Manifest{
		Verification: &schema.VerificationManifest{
			Duration: 5 * time.Minute,
		},
	}
	job := &Job{manifest: manifest}

	result := job.Manifest()

	require.Equal(t, manifest, result)
}

// Expectation: Nil should be returned when no manifest exists.
func Test_Job_Manifest_NoManifest_Success(t *testing.T) {
	t.Parallel()

	job := &Job{manifest: nil}

	result := job.Manifest()

	require.Nil(t, result)
}

// Expectation: The par2 path should be returned correctly.
func Test_Job_Par2Path_Success(t *testing.T) {
	t.Parallel()

	expectedPath := "/data/test" + schema.Par2Extension
	job := &Job{par2Path: expectedPath}

	result := job.Par2Path()

	require.Equal(t, expectedPath, result)
}

// Expectation: An empty string should be returned when no path is set.
func Test_Job_Par2Path_Empty_Success(t *testing.T) {
	t.Parallel()

	job := &Job{par2Path: ""}

	result := job.Par2Path()

	require.Empty(t, result)
}

// Expecation: The correct priority should be returned.
func Test_Job_queuePriority_NoManifest_Success(t *testing.T) {
	t.Parallel()

	job := &Job{manifest: nil}

	priority := job.queuePriority()

	require.Equal(t, prioNoManifest, priority)
}

// Expecation: The correct priority should be returned.
func Test_Job_queuePriority_NoVerification_Success(t *testing.T) {
	t.Parallel()

	job := &Job{
		manifest: &schema.Manifest{},
	}

	priority := job.queuePriority()

	require.Equal(t, prioNoVerification, priority)
}

// Expecation: The correct priority should be returned.
func Test_Job_queuePriority_NeedsRepair_Success(t *testing.T) {
	t.Parallel()

	job := &Job{
		manifest: &schema.Manifest{
			Verification: &schema.VerificationManifest{
				RepairNeeded: true,
			},
		},
	}

	priority := job.queuePriority()

	require.Equal(t, prioNeedsRepair, priority)
}

// Expecation: The correct priority should be returned.
func Test_Job_queuePriority_Normal_Success(t *testing.T) {
	t.Parallel()

	job := &Job{
		manifest: &schema.Manifest{
			Verification: &schema.VerificationManifest{
				RepairNeeded: false,
			},
		},
	}

	priority := job.queuePriority()

	require.Equal(t, prioOther, priority)
}

// Expectation: A zero time should be returned when no manifest exists.
func Test_Job_lastVerified_NoManifest_Success(t *testing.T) {
	t.Parallel()

	job := &Job{manifest: nil}

	t0 := job.lastVerified()

	require.Zero(t, t0)
}

// Expectation: The correct time should be returned when a manifest exists.
func Test_Job_lastVerified_WithVerification_Success(t *testing.T) {
	t.Parallel()

	now := time.Now()
	job := &Job{
		manifest: &schema.Manifest{
			Verification: &schema.VerificationManifest{
				Time: now,
			},
		},
	}

	t0 := job.lastVerified()

	require.Equal(t, now, t0)
}

// Expectation: A zero duration should be returned when no manifest exists.
func Test_Job_lastDuration_NoManifest_Success(t *testing.T) {
	t.Parallel()

	job := &Job{manifest: nil}

	duration := job.lastDuration()

	require.Zero(t, duration)
}

// Expectation: The correct duration should be returned when a manifest exists.
func Test_Job_lastDuration_WithVerification_Success(t *testing.T) {
	t.Parallel()

	job := &Job{
		manifest: &schema.Manifest{
			Verification: &schema.VerificationManifest{
				Duration: 5 * time.Minute,
			},
		},
	}

	duration := job.lastDuration()

	require.Equal(t, 5*time.Minute, duration)
}

// Expectation: A question mark should be printed if no manifest exists.
func Test_Job_lastDurationStr_NoManifest_Success(t *testing.T) {
	t.Parallel()

	job := &Job{
		manifest: nil,
	}

	result := job.lastDurationStr()

	require.Equal(t, "?", result)
}

// Expectation: A question mark should be printed if no verification exists.
func Test_Job_lastDurationStr_NoVerification_Success(t *testing.T) {
	t.Parallel()

	job := &Job{
		manifest: &schema.Manifest{},
	}

	result := job.lastDurationStr()

	require.Equal(t, "?", result)
}

// Expectation: The correct duration string should be returned for the duration.
func Test_Job_lastDurationStr_WithVerification_Success(t *testing.T) {
	t.Parallel()

	job := &Job{
		manifest: &schema.Manifest{
			Verification: &schema.VerificationManifest{
				Duration: 5 * time.Minute,
			},
		},
	}

	result := job.lastDurationStr()

	require.NotEqual(t, "?", result)
	require.Equal(t, "5m0s", result)
}

// Expectation: All jobs should be returned when no --age is given.
func Test_filterByAge_NoMinAge_Success(t *testing.T) {
	t.Parallel()

	jobs := []*Job{
		{par2Path: "/data/test1" + schema.Par2Extension},
		{par2Path: "/data/test2" + schema.Par2Extension},
	}

	filtered := filterByAge(jobs, 0)

	require.Len(t, filtered, 2)
}

// Expectation: The relevant jobs should be returned when --age is given.
func Test_filterByAge_WithMinAge_Success(t *testing.T) {
	t.Parallel()

	oldTime := time.Now().Add(-48 * time.Hour)
	recentTime := time.Now().Add(-1 * time.Hour)

	jobs := []*Job{
		{
			par2Path: "/data/old" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Time: oldTime,
				},
			},
		},
		{
			par2Path: "/data/recent" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Time: recentTime,
				},
			},
		},
	}

	filtered := filterByAge(jobs, 24*time.Hour)

	require.Len(t, filtered, 1)
	require.Equal(t, "/data/old"+schema.Par2Extension, filtered[0].par2Path)
}

// Expectation: Jobs without manifest should always be returned.
func Test_filterByAge_NoVerification_Success(t *testing.T) {
	t.Parallel()

	jobs := []*Job{
		{
			par2Path: "/data/test" + schema.Par2Extension,
			manifest: nil,
		},
	}

	filtered := filterByAge(jobs, 24*time.Hour)

	require.Len(t, filtered, 1)
}

// Expectation: All jobs should be returned without given --duration.
func Test_filterByDuration_NoMaxDuration_Success(t *testing.T) {
	t.Parallel()

	jobs := []*Job{
		{par2Path: "/data/test1" + schema.Par2Extension},
		{par2Path: "/data/test2" + schema.Par2Extension},
	}

	filtered := filterByDuration(jobs, 0)

	require.Len(t, filtered, 2)
}

// Expectation: The first job should always be included regardless of duration.
func Test_filterByDuration_FirstJobAlwaysIncluded_Success(t *testing.T) {
	t.Parallel()

	jobs := []*Job{
		{
			par2Path: "/data/large" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration: 10 * time.Hour,
				},
			},
		},
	}

	filtered := filterByDuration(jobs, 1*time.Hour)

	require.Len(t, filtered, 1)
}

// Expectation: The --duration should be respected and not exceeded.
func Test_filterByDuration_FitsMultiple_Success(t *testing.T) {
	t.Parallel()

	jobs := []*Job{
		{
			par2Path: "/data/job1" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration: 30 * time.Minute,
				},
			},
		},
		{
			par2Path: "/data/job2" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration: 20 * time.Minute,
				},
			},
		},
		{
			par2Path: "/data/job3" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration: 15 * time.Minute,
				},
			},
		},
	}

	filtered := filterByDuration(jobs, 1*time.Hour)

	require.Len(t, filtered, 2)
}

// Expectation: Jobs with unknown duration should always be included regardless of max duration.
func Test_filterByDuration_UnknownDurationAlwaysIncluded_Success(t *testing.T) {
	t.Parallel()

	jobs := []*Job{
		{
			par2Path: "/data/job1" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration: 30 * time.Minute,
				},
			},
		},
		{
			par2Path: "/data/unknown1" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration: 0,
				},
			},
		},
		{
			par2Path: "/data/unknown2" + schema.Par2Extension,
			manifest: nil,
		},
		{
			par2Path: "/data/job2" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration: 45 * time.Minute,
				},
			},
		},
	}

	filtered := filterByDuration(jobs, 1*time.Hour)

	require.Len(t, filtered, 3)
	require.Equal(t, "/data/job1"+schema.Par2Extension, filtered[0].par2Path)
	require.Equal(t, "/data/unknown1"+schema.Par2Extension, filtered[1].par2Path)
	require.Equal(t, "/data/unknown2"+schema.Par2Extension, filtered[2].par2Path)
}

// Expectation: Unknown duration jobs should be included even when they would exceed max duration.
func Test_filterByDuration_UnknownDurationExceedingLimit_Success(t *testing.T) {
	t.Parallel()

	jobs := []*Job{
		{
			par2Path: "/data/job1" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration: 50 * time.Minute,
				},
			},
		},
		{
			par2Path: "/data/unknown" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration: 0,
				},
			},
		},
		{
			par2Path: "/data/job2" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Duration: 20 * time.Minute,
				},
			},
		},
	}

	filtered := filterByDuration(jobs, 1*time.Hour)

	require.Len(t, filtered, 2)
	require.Equal(t, "/data/job1"+schema.Par2Extension, filtered[0].par2Path)
	require.Equal(t, "/data/unknown"+schema.Par2Extension, filtered[1].par2Path)
}

// Expectation: The correct sorting should be done according to priority.
func Test_sortJobs_Success(t *testing.T) {
	t.Parallel()

	jobs := []*Job{
		{
			par2Path: "/data/normal" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					RepairNeeded: false,
				},
			},
		},
		{
			par2Path: "/data/needs-repair" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					RepairNeeded: true,
				},
			},
		},
		{
			par2Path: "/data/no-manifest" + schema.Par2Extension,
			manifest: nil,
		},
	}

	sortJobs(jobs)

	require.Equal(t, "/data/no-manifest"+schema.Par2Extension, jobs[0].par2Path)
	require.Equal(t, "/data/needs-repair"+schema.Par2Extension, jobs[1].par2Path)
	require.Equal(t, "/data/normal"+schema.Par2Extension, jobs[2].par2Path)
}

// Expectation: Jobs with the same priority should be sorted by last verified time.
func Test_sortJobs_SamePriority_SortByTime_Success(t *testing.T) {
	t.Parallel()

	oldTime := time.Now().Add(-48 * time.Hour)
	recentTime := time.Now().Add(-24 * time.Hour)

	jobs := []*Job{
		{
			par2Path: "/data/recent" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Time:         recentTime,
					RepairNeeded: false,
				},
			},
		},
		{
			par2Path: "/data/old" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Time:         oldTime,
					RepairNeeded: false,
				},
			},
		},
	}

	sortJobs(jobs)

	require.Equal(t, "/data/old"+schema.Par2Extension, jobs[0].par2Path)
	require.Equal(t, "/data/recent"+schema.Par2Extension, jobs[1].par2Path)
}

// Expectation: Jobs with the same priority and time should be sorted by path.
func Test_sortJobs_SamePriorityAndTime_SortByPath_Success(t *testing.T) {
	t.Parallel()

	sameTime := time.Now()

	jobs := []*Job{
		{
			par2Path: "/data/zebra" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Time:         sameTime,
					RepairNeeded: false,
				},
			},
		},
		{
			par2Path: "/data/apple" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Time:         sameTime,
					RepairNeeded: false,
				},
			},
		},
	}

	sortJobs(jobs)

	require.Equal(t, "/data/apple"+schema.Par2Extension, jobs[0].par2Path)
	require.Equal(t, "/data/zebra"+schema.Par2Extension, jobs[1].par2Path)
}

// Expectation: Complex sorting should respect priority first, then time, then path.
func Test_sortJobs_ComplexSorting_Success(t *testing.T) {
	t.Parallel()

	oldTime := time.Now().Add(-72 * time.Hour)
	midTime := time.Now().Add(-48 * time.Hour)
	recentTime := time.Now().Add(-24 * time.Hour)

	jobs := []*Job{
		{
			par2Path: "/data/normal-recent" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Time:         recentTime,
					RepairNeeded: false,
				},
			},
		},
		{
			par2Path: "/data/repair-old" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Time:         oldTime,
					RepairNeeded: true,
				},
			},
		},
		{
			par2Path: "/data/no-manifest" + schema.Par2Extension,
			manifest: nil,
		},
		{
			par2Path: "/data/normal-old" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Time:         oldTime,
					RepairNeeded: false,
				},
			},
		},
		{
			par2Path: "/data/repair-mid" + schema.Par2Extension,
			manifest: &schema.Manifest{
				Verification: &schema.VerificationManifest{
					Time:         midTime,
					RepairNeeded: true,
				},
			},
		},
	}

	sortJobs(jobs)

	// Priority order: no manifest, needs repair (by time), normal (by time)
	require.Equal(t, "/data/no-manifest"+schema.Par2Extension, jobs[0].par2Path)
	require.Equal(t, "/data/repair-old"+schema.Par2Extension, jobs[1].par2Path)
	require.Equal(t, "/data/repair-mid"+schema.Par2Extension, jobs[2].par2Path)
	require.Equal(t, "/data/normal-old"+schema.Par2Extension, jobs[3].par2Path)
	require.Equal(t, "/data/normal-recent"+schema.Par2Extension, jobs[4].par2Path)
}
