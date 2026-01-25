package verify

import (
	"context"
	"log/slog"

	"github.com/desertwitch/par2cron/internal/schema"
)

func (prog *Service) verificationLogger(ctx context.Context, job *Job, path any) *slog.Logger {
	logElems := []any{}

	logElems = append(logElems, "op", "verify")
	if path != nil {
		logElems = append(logElems, "path", path)
	}

	if job != nil {
		logElems = append(logElems, "job", job.par2Path)

		if ctx.Value(schema.PosKey) != nil {
			logElems = append(logElems, "job_position", ctx.Value(schema.PosKey))
		}
		if ctx.Value(schema.MposKey) != nil {
			logElems = append(logElems, "job_position_sub", ctx.Value(schema.MposKey))
		}
		if ctx.Value(schema.PrioKey) != nil {
			logElems = append(logElems, "job_priority", ctx.Value(schema.PrioKey))
		}

		logElems = append(logElems, "args", job.par2Args)
	}

	return prog.log.With(logElems...)
}
