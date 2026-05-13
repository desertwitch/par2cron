package schema

import "time"

const MetaVersion uint8 = 1

type JobMeta struct {
	Par2Path        string
	VerifyTime      time.Time     // mf.Verification
	VerifyDuration  time.Duration // mf.Verification
	CountCorrupted  int           // mf.Verification
	MetaVersion     uint8
	Walked          bool
	IsBundle        bool
	HasManifest     bool
	HasCreation     bool // mf.Creation
	HasVerification bool // mf.Verification
	RepairNeeded    bool // mf.Verification
	RepairPossible  bool // mf.Verification
}

func NewJobMeta(par2path string, mf *Manifest, isBundle bool) *JobMeta {
	meta := &JobMeta{MetaVersion: MetaVersion}
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
