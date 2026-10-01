package schema

import "time"

const MetaVersion uint8 = 1

type JobMeta struct {
	Par2Path        string
	VerifyTime      time.Time     // mf.Verification
	VerifyDuration  time.Duration // mf.Verification
	CountCorrupted  int           // mf.Verification
	MetaVersion     uint8
	IsBundle        bool
	HasManifest     bool
	HasCreation     bool // mf.Creation
	HasVerification bool // mf.Verification
	RepairNeeded    bool // mf.Verification
	RepairPossible  bool // mf.Verification
	MaybeEdited     bool // mf.Verification

	Saved  bool // Was committed to the cache (durable on disk)
	Walked bool // Was discovered by filesystem walk (exists on disk)
}

func NewJobMeta(par2path string, mf *Manifest, isBundle bool) *JobMeta {
	meta := &JobMeta{
		IsBundle:    isBundle,
		MetaVersion: MetaVersion,
		Par2Path:    par2path,
	}

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
			meta.MaybeEdited = mf.Verification.MaybeEdited
		}
	}

	return meta
}
