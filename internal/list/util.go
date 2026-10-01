package list

import "github.com/desertwitch/par2cron/internal/verify"

type jobStatus int

// Ordered by severity: most urgent first (used for sorting).
const (
	statusUnrepairable jobStatus = iota
	statusRepairable
	statusUnverified
	statusHealthy
)

func (s jobStatus) String() string {
	switch s {
	case statusUnrepairable:
		return "unrepairable"

	case statusRepairable:
		return "repairable"

	case statusUnverified:
		return "unverified"

	case statusHealthy:
		return "healthy"

	default:
		return "unknown"
	}
}

func statusOf(m *verify.JobMeta) jobStatus {
	switch {
	case !m.HasManifest || !m.HasVerification:
		return statusUnverified

	case m.RepairNeeded && !m.RepairPossible:
		return statusUnrepairable

	case m.RepairNeeded:
		return statusRepairable

	default:
		return statusHealthy
	}
}
