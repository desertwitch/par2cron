package schema

import (
	"context"
	"io"
	"io/fs"

	"github.com/desertwitch/par2cron/internal/bundle"
	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/spf13/afero"
)

type FilesystemWalker interface {
	Name() string
	WalkDir(root string, fn fs.WalkDirFunc) error
}

type CommandRunner interface {
	Run(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error
}

type Par2Handler interface {
	Parse(r io.ReadSeeker, checkMD5 bool) ([]par2.Set, error)
	ParseFile(fsys afero.Fs, path string, panicAsErr bool) (p *par2.File, e error)
}

type BundleHandler interface {
	Open(fsys afero.Fs, bundlePath string) (Bundle, error)
	Pack(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error
}

type CacheHandler interface {
	NewCache(fsys afero.Fs, cacheDir string, cacheName string) Cache
}

type Cache interface {
	All() []*JobMeta
	Get(key string) (*JobMeta, bool)
	Len() int
	Load() error
	PruneUnwalked() int
	ResetWalked()
	Save() error
	Set(key string, meta *JobMeta)
}

type Bundle interface {
	Close() error
	Entries() []bundle.IndexEntry
	ExtractEntry(e bundle.IndexEntry, w io.Writer) error
	IsRebuilt() bool
	Manifest() ([]byte, error)
	ManifestName() string
	Unpack(fsys afero.Fs, destDir string, strict bool) ([]string, error)
	Update(manifest []byte) error
	Validate(strict bool) error
	ValidateIndex() error
}

type OptionsValidatable interface {
	Validate() error
}

type OptionsPar2ArgsSettable interface {
	SetPar2Args(args []string)
}
