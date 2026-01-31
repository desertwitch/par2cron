package util

import (
	"errors"
	"time"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/spf13/afero"
)

type Par2IndexToManifestOptions struct {
	Time     time.Time
	Path     string
	Manifest *schema.Manifest
}

func Par2IndexToManifest(fsys afero.Fs, o Par2IndexToManifestOptions, log *logging.Logger) {
	f, err := par2.ParseFile(fsys, o.Path, true)
	if err != nil {
		var pe *par2.ParserPanicError

		if errors.As(err, &pe) {
			log.Warn("Panic while parsing PAR2 for par2cron manifest (report to developers)",
				"panic", pe.Value, "stack", string(pe.Stack))
		} else {
			log.Warn("Failed to parse PAR2 for par2cron manifest (will retry next run)",
				"error", err)
		}

		return
	}

	if len(f.Sets) == 0 {
		log.Warn("PAR2 file is syntactically valid, but seems to contain no datasets")
	}

	if o.Manifest.Par2Data == nil {
		o.Manifest.Par2Data = &schema.Par2DataManifest{}
	}
	o.Manifest.Par2Data.Time = o.Time
	o.Manifest.Par2Data.Index = f

	log.Debug("Parsed PAR2 file to manifest")
}
