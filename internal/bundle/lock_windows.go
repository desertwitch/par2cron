//go:build windows

package bundle

import "os"

type fileLock struct {
	f *os.File
}

func newFileLock(f *os.File) *fileLock {
	return &fileLock{f: f}
}

func (l *fileLock) lock() error   { return nil }
func (l *fileLock) unlock() error { return nil }
