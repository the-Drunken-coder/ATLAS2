package postgres

import (
	"github.com/anomalyco/atlas-core/internal/runtime/config"
	"github.com/anomalyco/atlas-core/internal/runtime/logging"
)

func loggerOrNop(logs ...*logging.Logger) *logging.Logger {
	if len(logs) > 0 && logs[0] != nil {
		return logs[0]
	}
	return logging.New(&config.Config{LogLevel: "error"}, "postgres")
}
