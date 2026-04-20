package util

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidscholberg/go-durationfmt"
	"github.com/desertwitch/par2cron/internal/bundle"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/spf13/afero"
)

const (
	UmaskFilePerm      os.FileMode   = 0o666
	ProcessKillTimeout time.Duration = 10 * time.Second
)

var _ schema.BundleHandler = (*BundleHandler)(nil)

type BundleHandler struct{}

func (b *BundleHandler) Open(fsys afero.Fs, bundlePath string) (schema.Bundle, error) { //nolint:ireturn
	return bundle.Open(fsys, bundlePath) //nolint:wrapcheck
}

func (b *BundleHandler) Pack(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
	return bundle.Pack(fsys, bundlePath, recoverySetID, manifest, files) //nolint:wrapcheck
}

type ResultTracker struct {
	Selected int
	Success  int
	Skipped  int
	Error    int
}

func NewResultTracker() ResultTracker {
	return ResultTracker{}
}

func IsPar2Index(path string) bool {
	if !EndsWithFold(path, schema.Par2Extension) {
		return false
	}

	lower := strings.ToLower(filepath.Base(path))

	return !strings.Contains(lower, schema.Par2VolPrefix)
}

func IsPar2Volume(path string) bool {
	if !EndsWithFold(path, schema.Par2Extension) {
		return false
	}

	lower := strings.ToLower(filepath.Base(path))

	return strings.Contains(lower, schema.Par2VolPrefix)
}

func EndsWithFold(s, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}

	return strings.EqualFold(s[len(s)-len(suffix):], suffix)
}

func FmtDur(d time.Duration) string {
	d = d.Round(time.Second)

	str, err := durationfmt.Format(d, "%d days, %h hours %m minutes %s seconds")
	if err != nil {
		return "?"
	}

	return str
}

func IsGlobRecursive(pattern string) bool {
	for _, n := range []string{"/", "**"} {
		if strings.Contains(pattern, n) {
			return true
		}
	}

	return false
}
