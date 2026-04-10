package schema

import (
	"context"
	"io"
	"io/fs"
)

// FilesystemWalker is an interface describing a filesystem walking function.
type FilesystemWalker interface {
	Name() string
	WalkDir(root string, fn fs.WalkDirFunc) error
}

type CommandRunner interface {
	Run(ctx context.Context, cmd string, args []string, workingDir string, stdout io.Writer, stderr io.Writer) error
}

type OptionsValidatable interface {
	Validate() error
}

type OptionsPar2ArgsSettable interface {
	SetPar2Args(args []string)
}
