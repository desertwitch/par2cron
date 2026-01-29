package schema

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"time"

	"github.com/desertwitch/par2cron/internal/par2"
)

const (
	ManifestVersion = "2"
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

func NewManifest(par2Name string) *Manifest {
	return &Manifest{
		ProgramVersion:  ProgramVersion,
		ManifestVersion: ManifestVersion,
		Name:            par2Name,
	}
}

type CreationManifest struct {
	Time     time.Time     `json:"time"`
	Args     []string      `json:"args"`
	Duration time.Duration `json:"duration_ns"`
	Elements []FsElement   `json:"elements,omitempty"`
	PAR2     *par2.Archive `json:"par2,omitempty"`
}

func (c *CreationManifest) UnmarshalJSON(data []byte) error {
	type Alias CreationManifest

	aux := &struct {
		*Alias

		V1Files []FsElement `json:"files"`
	}{
		Alias: (*Alias)(c),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if len(c.Elements) == 0 && len(aux.V1Files) > 0 {
		c.Elements = aux.V1Files
	}

	return nil
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
	PAR2           *par2.Archive `json:"par2,omitempty"`
}

type RepairManifest struct {
	Count    int           `json:"count"`
	Time     time.Time     `json:"time"`
	Args     []string      `json:"args"`
	ExitCode int           `json:"exit_code"`
	Duration time.Duration `json:"duration_ns"`
	PAR2     *par2.Archive `json:"par2,omitempty"`
}

type FsElement struct {
	Path string `json:"-"` // Never export this to JSON.

	Name    string      `json:"name"`
	Size    int64       `json:"size"`
	Mode    fs.FileMode `json:"mode"`
	ModTime time.Time   `json:"mod_time"`
}
