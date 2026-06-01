package tool

import (
	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/spf13/afero"
)

type Service struct {
	fsys afero.Fs

	log     *logging.Logger
	bundler schema.BundleHandler
	par2er  schema.Par2Handler
}

func NewService(fsys afero.Fs, log *logging.Logger, bundler schema.BundleHandler, par2er schema.Par2Handler) *Service {
	return &Service{
		fsys:    fsys,
		log:     log.With("op", "tool"),
		bundler: bundler,
		par2er:  par2er,
	}
}
