package verify

import (
	"sort"
	"time"

	"github.com/desertwitch/par2cron/internal/schema"
)

type Stats struct {
	JobCount         int
	UnknownCount     int
	KnownCount       int
	Unverifieds      int
	Healthies        int
	Repairables      int
	Unrepairables    int
	AvgDuration      time.Duration
	TotalDuration    time.Duration
	LargestJob       *Job
	LargestDuration  time.Duration
	LastVerification time.Time
}

func (prog *Service) Stats(jobs []*Job) Stats {
	js := Stats{}

	js.JobCount = len(jobs)
	for _, job := range jobs {
		est := job.lastDuration()
		if est > 0 {
			js.TotalDuration += est
			js.KnownCount++
		}
		if est > js.LargestDuration {
			js.LargestDuration = est
			js.LargestJob = job
		}

		switch {
		case job.manifest == nil || job.manifest.Verification == nil:
			js.Unverifieds++

		case job.manifest.Verification.RepairNeeded && job.manifest.Verification.RepairPossible:
			js.Repairables++

		case job.manifest.Verification.RepairNeeded && !job.manifest.Verification.RepairPossible:
			js.Unrepairables++

		default:
			js.Healthies++
		}

		if job.manifest != nil && job.manifest.Verification != nil {
			if job.manifest.Verification.Time.After(js.LastVerification) {
				js.LastVerification = job.manifest.Verification.Time
			}
		}
	}

	js.UnknownCount = len(jobs) - js.KnownCount
	if js.KnownCount > 0 {
		js.AvgDuration = js.TotalDuration / time.Duration(js.KnownCount)
	}

	return js
}

func (job *Job) Manifest() *schema.Manifest {
	return job.manifest
}

func (job *Job) Par2Path() string {
	return job.par2Path
}

func (job *Job) queuePriority() int {
	switch {
	case job.manifest == nil:
		return prioNoManifest // No manifest.

	case job.manifest.Verification == nil:
		return prioNoVerification // Manifest, but no verification.

	case job.manifest.Verification.RepairNeeded:
		return prioNeedsRepair // PAR2 needing repair.

	default:
		return prioOther // Normal, sorted by verification age.
	}
}

func (job *Job) lastVerified() time.Time {
	if job.manifest == nil || job.manifest.Verification == nil {
		return time.Time{} // Zero time sorts first (oldest).
	}

	return job.manifest.Verification.Time
}

func (job *Job) lastDuration() time.Duration {
	if job.manifest == nil || job.manifest.Verification == nil {
		return 0
	}

	return job.manifest.Verification.Duration
}

func (job *Job) lastDurationStr() string {
	if job.manifest == nil || job.manifest.Verification == nil {
		return "?"
	}

	return job.manifest.Verification.Duration.String()
}

func filterByAge(jobs []*Job, minAge time.Duration) []*Job {
	if len(jobs) == 0 || minAge <= 0 {
		return jobs
	}

	now := time.Now()
	filtered := make([]*Job, 0, len(jobs))

	for _, job := range jobs {
		// Always include jobs with no manifest/no verification.
		// This is to get the first verification as soon as possible.
		if job.manifest == nil || job.manifest.Verification == nil {
			filtered = append(filtered, job)

			continue
		}

		// Otherwise include if last verification is older than minAge.
		age := now.Sub(job.manifest.Verification.Time)
		if age >= minAge {
			filtered = append(filtered, job)
		}
	}

	return filtered
}

func sortJobs(jobs []*Job) {
	sort.Slice(jobs, func(i, j int) bool {
		pi := jobs[i].queuePriority()
		pj := jobs[j].queuePriority()

		if pi != pj {
			return pi < pj // Sort by priority.
		}

		ti := jobs[i].lastVerified()
		tj := jobs[j].lastVerified()

		if !ti.Equal(tj) {
			return ti.Before(tj) // Sort by time (fallback).
		}

		return jobs[i].par2Path < jobs[j].par2Path // Sort by path (fallback).
	})
}

func filterByDuration(jobs []*Job, maxDuration time.Duration) []*Job {
	if len(jobs) == 0 || maxDuration <= 0 {
		return jobs
	}

	// Always take first job regardless of duration.
	// This is to ensure there's no starvation in the queue
	// if any job exceeds maxDuration and never gets picked.
	selected := []*Job{jobs[0]}
	total := jobs[0].lastDuration()

	// Fit remaining jobs within maxDuration.
	for _, job := range jobs[1:] {
		est := job.lastDuration()

		if est == 0 {
			// Always take jobs with no manifest/unknown duration.
			// This is to get a known duration set as soon as possible.
			selected = append(selected, job)

			continue
		}

		if total+est > maxDuration {
			// Otherwise try next (might be smaller).
			continue
		}

		selected = append(selected, job)
		total += est
	}

	return selected
}
