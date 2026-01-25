package schema

import (
	"context"
	"io/fs"
	"time"
)

const (
	ManifestVersion = "1"
)

type Manifest struct {
	ProgramVersion  string                `json:"program_version"`
	ManifestVersion string                `json:"manifest_version"`
	Name            string                `json:"name"`
	SHA256          string                `json:"sha256"`
	Creation        *CreationManifest     `json:"creation,omitempty"`
	Verification    *VerificationManifest `json:"verification,omitempty"`
	Repair          *RepairManifest       `json:"repair,omitempty"`
}

func NewManifest(ctx context.Context, par2Name string) *Manifest {
	var programVersion string
	if v, ok := ctx.Value(VersionKey).(string); ok {
		programVersion = v
	}

	return &Manifest{
		ProgramVersion:  programVersion,
		ManifestVersion: ManifestVersion,
		Name:            par2Name,
	}
}

type CreationManifest struct {
	Time       time.Time       `json:"time"`
	Args       []string        `json:"args"`
	Files      []ProtectedFile `json:"files"`
	FilesCount int             `json:"files_count"`
	Duration   time.Duration   `json:"duration_ns"`
}

type VerificationManifest struct {
	Count          int           `json:"count"`
	CountCorrupted int           `json:"count_corrupted"`
	Time           time.Time     `json:"time"`
	Args           []string      `json:"args"`
	ExitCode       int           `json:"exit_code"`
	RepairNeeded   bool          `json:"repair_needed"`
	RepairPossible bool          `json:"repair_possible"`
	Duration       time.Duration `json:"duration_ns"`
}

type RepairManifest struct {
	Count    int           `json:"count"`
	Time     time.Time     `json:"time"`
	Args     []string      `json:"args"`
	ExitCode int           `json:"exit_code"`
	Duration time.Duration `json:"duration_ns"`
}

type ProtectedFile struct {
	Path string `json:"-"` // Never export this to JSON.

	Name    string      `json:"name"`
	Size    int64       `json:"size"`
	Mode    fs.FileMode `json:"mode"`
	ModTime time.Time   `json:"mod_time"`
}
