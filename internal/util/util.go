package util

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidscholberg/go-durationfmt"
	"github.com/desertwitch/par2cron/internal/schema"
)

const (
	UmaskFilePerm      os.FileMode   = 0o666
	ProcessKillTimeout time.Duration = 10 * time.Second
)

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
