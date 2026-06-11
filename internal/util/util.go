package util

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/desertwitch/par2cron/internal/bundle"
	"github.com/desertwitch/par2cron/internal/cache"
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

func (b *BundleHandler) Open(ctx context.Context, fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
	return bundle.Open(ctx, fsys, bundlePath) //nolint:wrapcheck
}

func (b *BundleHandler) Pack(ctx context.Context, fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
	return bundle.Pack(ctx, fsys, bundlePath, recoverySetID, manifest, files) //nolint:wrapcheck
}

var _ schema.CacheHandler = (*GobCacheHandler)(nil)

type GobCacheHandler struct{}

func (GobCacheHandler) NewCache(fsys afero.Fs, cacheDir string, cacheName string) schema.Cache {
	return cache.NewGobCache(fsys, cacheDir, cacheName)
}

var _ schema.Par2Handler = (*Par2Handler)(nil)

type Par2Handler struct{}

func (h *Par2Handler) Parse(r io.ReadSeeker, checkMD5 bool) ([]par2.Set, error) {
	return par2.Parse(r, checkMD5) //nolint:wrapcheck
}

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
