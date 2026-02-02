package create

import (
	"context"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
)

func (prog *Service) creationLogger(ctx context.Context, job *Job, path any) *logging.Logger {
	logElems := []any{}

	if path != nil {
		logElems = append(logElems, "path", path)
	}

	if job != nil {
		logElems = append(logElems, "job", job.markerPath)

		if ctx.Value(schema.PosKey) != nil {
			logElems = append(logElems, "job_position", ctx.Value(schema.PosKey))
		}
		if ctx.Value(schema.MposKey) != nil {
			logElems = append(logElems, "job_position_sub", ctx.Value(schema.MposKey))
		}

		logElems = append(logElems,
			"args", job.par2Args,
			"glob", job.par2Glob,
			"mode", job.par2Mode,
			"hidden", job.hiddenFiles,
			"verify", job.par2Verify)
	}

	return prog.log.With(logElems...)
}

func (prog *Service) markerLogger(job any, key any, value any) *logging.Logger {
	logElems := []any{}

	logElems = append(logElems, "job", job)

	if key != nil {
		logElems = append(logElems, "key", key)
	}
	if value != nil {
		logElems = append(logElems, "value", value)
	}

	return prog.log.With(logElems...)
}

func (prog *Service) debugArgsModified(arg string, value string, before any, after any, wasReplaced bool, markerPath string) {
	if !wasReplaced {
		prog.log.Debug("Added argument to argument slice",
			"job", markerPath,
			"arg", arg,
			"value", value,
			"before", before,
			"after", after)
	} else {
		prog.log.Debug("Replaced argument in argument slice",
			"job", markerPath,
			"arg", arg,
			"value", value,
			"before", before,
			"after", after)
	}
}
