package repair

import (
	"context"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
)

func (prog *Service) repairLogger(ctx context.Context, job any, path any) *logging.Logger {
	logElems := []any{}

	if path != nil {
		logElems = append(logElems, "path", path)
	}

	if job != nil {
		switch j := job.(type) {
		case *schema.JobMeta:
			logElems = append(logElems, "job", j.Par2Path)
			if ctx.Value(schema.PosKey) != nil {
				logElems = append(logElems, "job_position", ctx.Value(schema.PosKey))
			}
		case *JobMeta:
			logElems = append(logElems, "job", j.Par2Path)
			if ctx.Value(schema.PosKey) != nil {
				logElems = append(logElems, "job_position", ctx.Value(schema.PosKey))
			}
		case *Job:
			logElems = append(logElems, "job", j.par2Path)
			if ctx.Value(schema.PosKey) != nil {
				logElems = append(logElems, "job_position", ctx.Value(schema.PosKey))
			}
			logElems = append(logElems, "args", j.par2Args, "verify", j.par2Verify)
		}
	}

	return prog.log.With(logElems...)
}
