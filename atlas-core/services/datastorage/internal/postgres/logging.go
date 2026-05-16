package postgres

import (
	"github.com/anomalyco/atlas-core/services/shared/logging"
)

func loggerOrNop(logs ...*logging.Logger) *logging.Logger {
	for _, log := range logs {
		if log != nil {
			return log
		}
	}
	return logging.New("error", "atlas-datastorage", "postgres")
}
