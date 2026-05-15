package postgres

import (
	"github.com/anomalyco/atlas-core/services/shared/logging"
)

func loggerOrNop(logs ...*logging.Logger) *logging.Logger {
	if len(logs) > 0 && logs[0] != nil {
		return logs[0]
	}
	return logging.New("error", "atlas-datastorage", "postgres")
}
