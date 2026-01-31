package repair

import (
	"context"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
)

func (prog *Service) repairLogger(ctx context.Context, job *Job, path any) *logging.Logger {
	logElems := []any{}

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
