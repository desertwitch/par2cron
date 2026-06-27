package util

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/desertwitch/par2cron/internal/schema"
)

var (
	_ schema.CommandRunner = (*CtxRunner)(nil)
	_ io.Closer            = (*CtxRunner)(nil)
)

type RunnerOption func(*CtxRunner) error

func WithCgroup(path string) RunnerOption {
	return func(r *CtxRunner) error {
		cleaned := filepath.Clean(path)

		f, err := os.Open(cleaned)
		if err != nil {
			return fmt.Errorf("failed to open cgroup: %w", err)
		}
		r.CgroupFile = f

		return nil
	}
}

type CtxRunner struct {
	CgroupFile *os.File
}

func NewCtxRunner(opts ...RunnerOption) (*CtxRunner, error) {
	r := &CtxRunner{}

	for _, o := range opts {
		if err := o(r); err != nil {
			return nil, err
		}
	}

	return r, nil
}

func (r *CtxRunner) Close() error {
	if r.CgroupFile != nil {
		return r.CgroupFile.Close() //nolint:wrapcheck
	}

	return nil
}

func (r *CtxRunner) Run(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
	c := exec.CommandContext(ctx, cmd, args...)

	c.Dir = workingDir
	c.Stdout = stdout
	c.Stderr = stderr

	c.Cancel = func() error {
		return c.Process.Signal(os.Interrupt)
	}
	c.WaitDelay = ProcessKillTimeout

	if r.CgroupFile != nil {
		c.SysProcAttr = &syscall.SysProcAttr{
			UseCgroupFD: true,
			CgroupFD:    int(r.CgroupFile.Fd()),
		}
	}

	return c.Run() //nolint:wrapcheck
}
