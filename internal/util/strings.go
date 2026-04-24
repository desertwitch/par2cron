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

	return !IsPar2Volume(path)
}

func IsPar2Volume(path string) bool {
	if !EndsWithFold(path, schema.Par2Extension) {
		return false
	}

	name := strings.ToLower(filepath.Base(path))
	extLen := len(schema.Par2Extension)

	// split "<stem>.par2"
	stem := name[:len(name)-extLen]

	// must contain ".vol" and have non-empty root before it
	vol := strings.LastIndex(stem, schema.Par2VolPrefix)
	if vol <= 0 {
		return false
	}

	// suffix after ".vol" must be "<start>+<count>" with digits only
	mid := stem[vol+len(schema.Par2VolPrefix):]
	plus := strings.IndexByte(mid, '+')
	if plus <= 0 || plus >= len(mid)-1 || strings.IndexByte(mid[plus+1:], '+') != -1 {
		return false
	}

	return isDigits(mid[:plus]) && isDigits(mid[plus+1:])
}

func IsPar2Bundle(path string) bool {
	return EndsWithFold(path, schema.BundleExtension+schema.Par2Extension)
}

// IsPar2SetMember reports whether candidate is a canonical member of the same
// PAR2 set as par2Name, using case-insensitive basename matching.
//
// Canonical members are:
//   - <root>.par2
//   - <root>.p2c.par2
//   - <root>.vol<start>+<count>.par2 (strict numeric form)
//
// If par2Name is a bundle (<root>.p2c.par2), matching is normalized to <root>.
func IsPar2SetMember(par2Name, candidate string) bool {
	base := strings.ToLower(TrimSuffixFold(filepath.Base(par2Name), schema.Par2Extension))
	root := strings.TrimSuffix(base, schema.BundleExtension)
	name := strings.ToLower(filepath.Base(candidate))

	if root == "" || root == "." || root == ".." {
		return false
	}
	if name == "." || name == ".." {
		return false
	}

	if name == root+schema.Par2Extension { // file.par2
		return true
	}
	if name == root+schema.BundleExtension+schema.Par2Extension { // file.p2c.par2
		return true
	}

	prefix := root + schema.Par2VolPrefix // file.vol
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, schema.Par2Extension) {
		return false
	}

	mid := name[len(prefix) : len(name)-len(schema.Par2Extension)]
	plus := strings.IndexByte(mid, '+')
	if plus <= 0 || plus >= len(mid)-1 || strings.IndexByte(mid[plus+1:], '+') != -1 {
		return false
	}

	return isDigits(mid[:plus]) && isDigits(mid[plus+1:])
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
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
