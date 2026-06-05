package verify

import (
	"sort"
	"time"

	"github.com/desertwitch/par2cron/internal/schema"
)

type Stats struct {
	JobCount          int
	UnknownCount      int
	KnownCount        int
	Unverifieds       int
	Healthies         int
	Repairables       int
	Unrepairables     int
	AvgDuration       time.Duration
	TotalDuration     time.Duration
	LargestJob        *schema.JobMeta
	LargestDuration   time.Duration
	FirstVerification time.Time
	LastVerification  time.Time
}

func (prog *Service) Stats(metas []*JobMeta) Stats {
	js := Stats{}

	js.JobCount = len(metas)
	for _, meta := range metas {
		est := meta.lastDuration()
		if est > 0 {
			js.TotalDuration += est
			js.KnownCount++
		}
		if est > js.LargestDuration {
			js.LargestDuration = est
			js.LargestJob = meta.JobMeta
		}

		switch {
		case !meta.HasManifest || !meta.HasVerification:
			js.Unverifieds++

		case meta.RepairNeeded && meta.RepairPossible:
			js.Repairables++

		case meta.RepairNeeded && !meta.RepairPossible:
			js.Unrepairables++

		default:
			js.Healthies++
		}

		if meta.HasVerification && !meta.VerifyTime.IsZero() {
			if js.FirstVerification.IsZero() || meta.VerifyTime.Before(js.FirstVerification) {
				js.FirstVerification = meta.VerifyTime
			}
			if meta.VerifyTime.After(js.LastVerification) {
				js.LastVerification = meta.VerifyTime
			}
		}
	}

	js.UnknownCount = len(metas) - js.KnownCount
	if js.KnownCount > 0 {
		js.AvgDuration = js.TotalDuration / time.Duration(js.KnownCount)
	}

	return js
}

func (meta *JobMeta) queuePriority() int {
	switch {
	case !meta.HasManifest:
		return prioNoManifest // No manifest.

	case !meta.HasVerification:
		return prioNoVerification // Manifest, but no verification.

	case meta.RepairNeeded:
		return prioNeedsRepair // PAR2 needing repair.

	default:
		return prioOther // Normal, sorted by verification age.
	}
}

func (meta *JobMeta) lastVerified() time.Time {
	if !meta.HasManifest || !meta.HasVerification {
		return time.Time{} // Zero time sorts first (oldest).
	}

	return meta.VerifyTime
}

func (meta *JobMeta) lastVerifiedStr() string {
	if !meta.HasManifest || !meta.HasVerification {
		return ""
	}

	return meta.VerifyTime.String()
}

func (meta *JobMeta) lastDuration() time.Duration {
	if !meta.HasManifest || !meta.HasVerification {
		return 0
	}

	return meta.VerifyDuration
}

func (meta *JobMeta) lastDurationStr() string {
	if !meta.HasManifest || !meta.HasVerification {
		return ""
	}

	return meta.VerifyDuration.String()
}

func filterByAge(metas []*JobMeta, minAge time.Duration) []*JobMeta {
	if len(metas) == 0 || minAge <= 0 {
		return metas
	}

	now := time.Now()
	filtered := make([]*JobMeta, 0, len(metas))

	for _, meta := range metas {
		// Always include jobs with no manifest/no verification.
		// This is to get the first verification as soon as possible.
		if !meta.HasManifest || !meta.HasVerification {
			filtered = append(filtered, meta)

			continue
		}

		// Otherwise include if last verification is older than minAge.
		age := now.Sub(meta.VerifyTime)
		if age >= minAge {
			filtered = append(filtered, meta)
		}
	}

	return filtered
}

func sortJobs(metas []*JobMeta) {
	sort.Slice(metas, func(i, j int) bool {
		pi := metas[i].queuePriority()
		pj := metas[j].queuePriority()

		if pi != pj {
			return pi < pj // Sort by priority.
		}

		ti := metas[i].lastVerified()
		tj := metas[j].lastVerified()

		if !ti.Equal(tj) {
			return ti.Before(tj) // Sort by time (fallback).
		}

		return metas[i].Par2Path < metas[j].Par2Path // Sort by path (fallback).
	})
}

func filterByDuration(metas []*JobMeta, maxDuration time.Duration) []*JobMeta {
	if len(metas) == 0 || maxDuration <= 0 {
		return metas
	}

	// Always take first job regardless of duration.
	// This is to ensure there's no starvation in the queue
	// if any job exceeds maxDuration and never gets picked.
	selected := []*JobMeta{metas[0]}
	total := metas[0].lastDuration()

	// Fit remaining jobs within maxDuration.
	for _, meta := range metas[1:] {
		est := meta.lastDuration()

		if est == 0 {
			// Always take jobs with no manifest/unknown duration.
			// This is to get a known duration set as soon as possible.
			selected = append(selected, meta)

			continue
		}

		if total+est > maxDuration {
			// Otherwise try next (might be smaller).
			continue
		}

		selected = append(selected, meta)
		total += est
	}

	return selected
}

func knownDuration(metas []*JobMeta) time.Duration {
	var duration time.Duration

	for _, meta := range metas {
		if meta.HasManifest && meta.HasVerification && meta.VerifyDuration > 0 {
			duration += meta.VerifyDuration
		}
	}

	return duration
}
