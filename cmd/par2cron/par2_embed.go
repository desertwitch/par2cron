//go:build embed_par2

package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed par2
var par2Binary []byte

// setupPar2 extracts the bundled par2cmdline binary to a temporary directory,
// makes it executable, and prepends it to PATH so that par2cron can run fully
// self-contained. This makes sense only for static builds of par2cmdline.
//
// This function is only included in builds that use the "embed_par2" build tag,
// which requires a valid "par2" binary at "./cmd/par2cron/par2". Default builds
// however do not bundle the "par2" binary into the compiled par2cron executable.
//
// Call the returned cleanup function once all PAR2 operations are complete.
func setupPar2() (func(), error) {
	dir, err := os.MkdirTemp("", "par2cron-*")
	if err != nil {
		return func() {}, fmt.Errorf("failed to mktmp: %w", err)
	}

	cleanup := func() { _ = os.RemoveAll(dir) }

	p := filepath.Join(dir, "par2")
	if err := os.WriteFile(p, par2Binary, 0o755); err != nil { //nolint:mnd
		cleanup()

		return func() {}, fmt.Errorf("failed to extract: %w", err)
	}

	err = os.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	if err != nil {
		cleanup()

		return func() {}, fmt.Errorf("failed to setenv: %w", err)
	}

	return cleanup, nil
}
