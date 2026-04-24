package util

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/davidscholberg/go-durationfmt"
	"github.com/desertwitch/par2cron/internal/schema"
)

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

func IsPar2Bundle(path string) bool {
	return EndsWithFold(path, schema.BundleExtension+schema.Par2Extension)
}

func EndsWithFold(s, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}

	return strings.EqualFold(s[len(s)-len(suffix):], suffix)
}

func TrimSuffixFold(s, suffix string) string {
	if len(s) >= len(suffix) && strings.EqualFold(s[len(s)-len(suffix):], suffix) {
		return s[:len(s)-len(suffix)]
	}

	return s
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
