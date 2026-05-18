package create

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/desertwitch/par2cron/internal/testutil"
)

func Benchmark_Enumerate_HotPath(b *testing.B) {
	entries := make([]testutil.FakeDirEntry, 1000)
	for i := range entries {
		entries[i] = testutil.FakeDirEntry{EntryName: fmt.Sprintf("file_%d.txt", i)}
	}

	ctx := context.Background()
	prog := &Service{
		walker: &testutil.FakeWalker{Entries: entries},
	}

	for b.Loop() {
		_ = prog.walker.WalkDir("/root", func(path string, d fs.DirEntry, err error) error {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("context error: %w", err)
			}
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasPrefix(d.Name(), createMarkerPathPrefix) {
				return nil
			}

			return nil
		})
	}
}
