package util

import (
	"fmt"
	"log/slog"
	"os"
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
	Success int
	Skipped int
	Error   int

	log *slog.Logger
}

func NewResultTracker(log *slog.Logger) *ResultTracker {
	return &ResultTracker{
		log: log,
	}
}

func (t *ResultTracker) PrintCompletionInfo(selectedCount int) {
	processed := t.Success + t.Error + t.Skipped

	t.log.Info(
		fmt.Sprintf("Operation complete (%d/%d jobs processed)",
			processed, selectedCount),
		"successCount", t.Success,
		"skipCount", t.Skipped,
		"errorCount", t.Error,
		"processedCount", processed,
		"selectedCount", selectedCount,
	)
}

// Ptr converts a value of type [T] to a pointer of type [*T].
func Ptr[T any](v T) *T {
	return &v
}

func IsPar2Base(path string) bool {
	lower := strings.ToLower(path)

	if !strings.HasSuffix(lower, schema.Par2Extension) {
		return false
	}

	return !strings.Contains(lower, ".vol")
}

func FmtDur(d time.Duration) string {
	d = d.Round(time.Second)

	str, err := durationfmt.Format(d, "%d days, %h hours %m minutes %s seconds")
	if err != nil {
		return "?"
	}

	return str
}
