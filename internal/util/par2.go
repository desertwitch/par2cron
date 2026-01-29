package util

import (
	"runtime/debug"
	"sync"

	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/spf13/afero"
)

func ParsePar2To(target **par2.Archive, fsys afero.Fs, path string, log func(msg string, args ...any)) {
	var wg sync.WaitGroup
	wg.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				if log != nil {
					log("Panic parsing PAR2 for par2cron manifest (report to developers)",
						"panic", r, "stack", string(debug.Stack()))
				}
				if target != nil {
					*target = nil
				}
			}
		}()
		if parsed, err := par2.ParseFile(fsys, path); err != nil {
			if log != nil {
				log("Failed to parse created PAR2 for par2cron manifest", "error", err)
			}
			if target != nil {
				*target = nil
			}
		} else if target != nil {
			*target = parsed
		}
	})
	wg.Wait()
}
