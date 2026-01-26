package testutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/afero"
)

type SafeBuffer struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func (sb *SafeBuffer) Bytes() []byte {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	return sb.buf.Bytes()
}

func (sb *SafeBuffer) Write(p []byte) (n int, err error) { //nolint:nonamedreturns
	sb.mu.Lock()
	defer sb.mu.Unlock()

	return sb.buf.Write(p)
}

func (sb *SafeBuffer) String() string {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	return sb.buf.String()
}

func (sb *SafeBuffer) Reset() {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.buf.Reset()
}

func (sb *SafeBuffer) Len() int {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	return sb.buf.Len()
}

type MockRunner struct {
	RunFunc func(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error
}

func (m *MockRunner) Run(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, cmd, args, workingDir, stdout, stderr)
	}

	return nil
}

// FailingOpenFs wraps an afero.Fs and fails Open calls for files matching failPattern.
type FailingOpenFs struct {
	afero.Fs

	FailPattern string
}

func (f *FailingOpenFs) Open(name string) (afero.File, error) {
	if strings.Contains(name, f.FailPattern) {
		return nil, errors.New("simulated open failure")
	}

	return f.Fs.Open(name)
}

type FailingStatFs struct {
	afero.Fs

	FailPattern string
}

func (f *FailingStatFs) Stat(name string) (os.FileInfo, error) {
	if strings.Contains(name, f.FailPattern) {
		return nil, errors.New("stat failed")
	}

	return f.Fs.Stat(name)
}

type FailingRenameFs struct {
	afero.Fs

	FailPattern string
}

func (f *FailingRenameFs) Rename(oldname, newname string) error {
	if strings.Contains(oldname, f.FailPattern) || strings.Contains(newname, f.FailPattern) {
		return errors.New("rename failed")
	}

	return f.Fs.Rename(oldname, newname)
}

type FailingRemoveFs struct {
	afero.Fs

	FailSuffix string
}

func (f *FailingRemoveFs) Remove(name string) error {
	if strings.HasSuffix(name, f.FailSuffix) {
		return errors.New("permission denied: remove failed")
	}

	return f.Fs.Remove(name)
}

type FailingWriteFs struct {
	afero.Fs

	FailSuffix string
}

func (f *FailingWriteFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if strings.HasSuffix(name, f.FailSuffix) && (flag&os.O_CREATE != 0 || flag&os.O_WRONLY != 0) {
		return nil, errors.New("permission denied: open file failed")
	}

	return f.Fs.OpenFile(name, flag, perm)
}

func (f *FailingWriteFs) Create(name string) (afero.File, error) {
	if strings.HasSuffix(name, f.FailSuffix) {
		return nil, errors.New("permission denied: create file failed")
	}

	return f.Fs.Create(name)
}

func CreateExitError(t *testing.T, ctx context.Context, code int) error {
	t.Helper()

	if code < 0 {
		panic("exit code cannot be < 0")
	}

	cmd := exec.CommandContext(ctx, //nolint:gosec
		"sh", "-c", fmt.Sprintf("exit %d", code))
	cmd.WaitDelay = 5 * time.Second //nolint:mnd

	return cmd.Run()
}
