package repair

import (
	"context"
	"log/slog"

	"github.com/desertwitch/par2cron/internal/schema"
)

func (prog *Service) repairLogger(ctx context.Context, job *Job, path any) *slog.Logger {
	logElems := []any{}

	logElems = append(logElems, "op", "repair")
	if path != nil {
		logElems = append(logElems, "path", path)
	}

	if job != nil {
		logElems = append(logElems, "job", job.par2Path)

		if ctx.Value(schema.PosKey) != nil {
			logElems = append(logElems, "job_position", ctx.Value(schema.PosKey))
		}

		logElems = append(logElems, "args", job.par2Args, "verify", job.par2Verify)
	}

	return prog.log.With(logElems...)
}
