package cache

import (
	"time"

	"github.com/desertwitch/par2cron/internal/schema"
)

type JobMeta struct {
	Par2Path        string
	IsBundle        bool
	HasManifest     bool
	HasCreation     bool
	HasVerification bool

	VerifyTime     time.Time
	VerifyDuration time.Duration

	RepairNeeded   bool
	RepairPossible bool
	CountCorrupted int
}

func NewJobMeta(par2path string, mf *schema.Manifest, isBundle bool) *JobMeta {
	meta := &JobMeta{}
	meta.Par2Path = par2path
	meta.IsBundle = isBundle

	if mf != nil {
		meta.HasManifest = true

		if mf.Creation != nil {
			meta.HasCreation = true
		}
		if mf.Verification != nil {
			meta.HasVerification = true
			meta.VerifyTime = mf.Verification.Time
			meta.VerifyDuration = mf.Verification.Duration
			meta.RepairNeeded = mf.Verification.RepairNeeded
			meta.RepairPossible = mf.Verification.RepairPossible
			meta.CountCorrupted = mf.Verification.CountCorrupted
		}
	}

	return meta
}
