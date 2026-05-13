package cache

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/klauspost/compress/zstd"
	"github.com/spf13/afero"
)

type GobCache struct {
	fsys afero.Fs
	path string

	items map[string]*schema.JobMeta
}

// NewGobCache creates a new GobCache with a hashed filename derived from cacheName.
func NewGobCache(fsys afero.Fs, cacheDir string, cacheName string) *GobCache {
	hash := sha256.Sum256([]byte(cacheName))
	cacheName = hex.EncodeToString(hash[:8]) + ".gob"
	cachePath := filepath.Join(cacheDir, cacheName)

	c := &GobCache{
		fsys:  fsys,
		path:  cachePath,
		items: make(map[string]*schema.JobMeta),
	}

	return c
}

// Len returns the number of entries in the cache.
func (c *GobCache) Len() int {
	return len(c.items)
}

// Get returns the JobMeta for the given key, or nil and false if not found.
func (c *GobCache) Get(key string) (*schema.JobMeta, bool) {
	meta, ok := c.items[key]
	if !ok {
		return nil, false
	}

	return meta, true
}

// Set adds or updates a JobMeta entry in the cache.
func (c *GobCache) Set(key string, meta *schema.JobMeta) {
	c.items[key] = meta
}

// Touch marks a key as walked in the current walk.
func (c *GobCache) Touch(key string) {
	if meta, ok := c.items[key]; ok {
		meta.Walked = true
	}
}

// ResetWalked sets all items of the cache to not seen.
func (c *GobCache) ResetWalked() {
	for _, meta := range c.items {
		meta.Walked = false
	}
}

// Prune removes all entries not seen in the current walk.
func (c *GobCache) Prune() int {
	pruned := 0

	for key, meta := range c.items {
		if !meta.Walked {
			delete(c.items, key)
			pruned++
		}
	}

	return pruned
}

// All returns all cached JobMeta pointers in no particular order.
func (c *GobCache) All() []*schema.JobMeta {
	result := make([]*schema.JobMeta, 0, len(c.items))

	for _, meta := range c.items {
		result = append(result, meta)
	}

	return result
}

// Load reads the cache from disk, streaming entries one at a time.
func (c *GobCache) Load() error {
	f, err := c.fsys.Open(c.path)
	if err != nil {
		return fmt.Errorf("failed to open: %w", err)
	}
	defer f.Close()

	if f, ok := f.(*os.File); ok {
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_SH); err != nil {
			return fmt.Errorf("failed to lock: %w", err)
		}
		defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck
	}

	zr, err := zstd.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to create zstd reader: %w", err)
	}
	defer zr.Close()

	dec := gob.NewDecoder(zr)

	var count int
	if err := dec.Decode(&count); err != nil {
		return fmt.Errorf("failed to decode count: %w", err)
	}

	c.items = make(map[string]*schema.JobMeta, count)
	for i := range count {
		var m schema.JobMeta

		if err := dec.Decode(&m); err != nil {
			return fmt.Errorf("failed to decode value %d: %w", i, err)
		}

		c.items[m.Par2Path] = &m
	}

	return nil
}

// Save writes the cache to disk, streaming entries one at a time.
func (c *GobCache) Save() error {
	f, err := c.fsys.OpenFile(c.path, os.O_CREATE|os.O_WRONLY, 0o666) //nolint:mnd
	if err != nil {
		return fmt.Errorf("failed to open: %w", err)
	}
	defer f.Close()

	if f, ok := f.(*os.File); ok {
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
			return fmt.Errorf("failed to lock: %w", err)
		}
		defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck
	}
	defer f.Sync() //nolint:errcheck

	if err := f.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate: %w", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}

	zw, err := zstd.NewWriter(f, zstd.WithEncoderLevel(zstd.SpeedFastest))
	if err != nil {
		return fmt.Errorf("failed to create zstd writer: %w", err)
	}
	defer zw.Close()

	enc := gob.NewEncoder(zw)

	count := len(c.items)
	if err := enc.Encode(count); err != nil {
		return fmt.Errorf("failed to encode count: %w", err)
	}

	for _, meta := range c.items {
		meta.Walked = false
		if err := enc.Encode(meta); err != nil {
			return fmt.Errorf("failed to encode value: %w", err)
		}
	}

	return nil
}
