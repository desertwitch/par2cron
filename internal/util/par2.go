package util

import (
	"errors"
	"log/slog"
	"time"

	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/spf13/afero"
)

type Par2ToManifestOptions struct {
	Time     time.Time
	Path     string
	Manifest *schema.Manifest
}

func Par2ToManifest(fsys afero.Fs, o Par2ToManifestOptions, log *slog.Logger) {
	archive, err := par2.ParseFile(fsys, o.Path, true)
	if err != nil {
		var pe *par2.ParserPanicError
		if errors.As(err, &pe) {
			log.Warn("Panic while parsing PAR2 for par2cron manifest (report to developers)",
				"panic", pe.Value, "stack", pe.Stack)
		} else {
			log.Warn("Failed to parse PAR2 for par2cron manifest (will retry next run)",
				"error", err)
		}

		return
	}

	if archive == nil || len(archive.Sets) == 0 {
		log.Warn("PAR2 file parsed as containing no datasets (will retry next run)")

		o.Manifest.Archive = nil

		return
	}

	if o.Manifest.Archive == nil {
		o.Manifest.Archive = &schema.ArchiveManifest{}
	}

	o.Manifest.Archive.Time = o.Time
	o.Manifest.Archive.Content = archive

	log.Debug("Succeeded to parse PAR2 to manifest")
}
