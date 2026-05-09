//go:build !windows

//nolint:wrapcheck
package bundle

import (
	"os"
	"syscall"
)

type fileLock struct {
	f *os.File
}

func newFileLock(f *os.File) *fileLock {
	return &fileLock{f: f}
}

func (l *fileLock) lock() error {
	return syscall.Flock(int(l.f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

func (l *fileLock) unlock() error {
	return syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
}
