package util

import (
	"os"
	"time"

	"github.com/desertwitch/par2cron/internal/bundle"
	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/spf13/afero"
)

const (
	UmaskFilePerm      os.FileMode   = 0o666
	ProcessKillTimeout time.Duration = 10 * time.Second
)

var _ schema.BundleHandler = (*BundleHandler)(nil)

type BundleHandler struct{}

func (b *BundleHandler) Open(fsys afero.Fs, bundlePath string) (schema.Bundle, error) { //nolint:ireturn
	return bundle.Open(fsys, bundlePath) //nolint:wrapcheck
}

func (b *BundleHandler) Pack(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
	return bundle.Pack(fsys, bundlePath, recoverySetID, manifest, files) //nolint:wrapcheck
}

var _ schema.Par2Handler = (*Par2Handler)(nil)

type Par2Handler struct{}

func (h *Par2Handler) ParseFile(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
	return par2.ParseFile(fsys, path, panicAsErr) //nolint:wrapcheck
}

type ResultTracker struct {
	Selected int
	Success  int
	Skipped  int
	Error    int
}

func NewResultTracker() ResultTracker {
	return ResultTracker{}
}
