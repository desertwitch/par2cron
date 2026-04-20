package schema

import (
	"context"
	"io"
	"io/fs"

	"github.com/desertwitch/par2cron/internal/bundle"
	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/spf13/afero"
)

// FilesystemWalker is an interface describing a filesystem walking function.
type FilesystemWalker interface {
	Name() string
	WalkDir(root string, fn fs.WalkDirFunc) error
}

type CommandRunner interface {
	Run(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error
}

type Par2Handler interface {
	ParseFile(fsys afero.Fs, path string, panicAsErr bool) (p *par2.File, e error)
}

type BundleHandler interface {
	Open(fsys afero.Fs, bundlePath string) (Bundle, error)
	Pack(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error
}

type Bundle interface {
	Close() error
	Manifest() ([]byte, error)
	Update(manifest []byte) error
	ValidateIndex() error
}

type OptionsValidatable interface {
	Validate() error
}

type OptionsPar2ArgsSettable interface {
	SetPar2Args(args []string)
}
