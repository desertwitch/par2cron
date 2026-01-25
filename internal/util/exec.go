package util

import (
	"context"
	"io"
	"os"
	"os/exec"

	"github.com/desertwitch/par2cron/internal/schema"
)

type CtxRunner struct{}

var _ schema.CommandRunner = (*CtxRunner)(nil)

func (CtxRunner) Run(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
	c := exec.CommandContext(ctx, cmd, args...)

	c.Dir = workingDir
	c.Stdout = stdout
	c.Stderr = stderr

	c.Cancel = func() error {
		return c.Process.Signal(os.Interrupt)
	}
	c.WaitDelay = ProcessKillTimeout

	return c.Run() //nolint:wrapcheck
}
