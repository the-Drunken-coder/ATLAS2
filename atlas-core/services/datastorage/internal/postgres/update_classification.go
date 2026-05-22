package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/services/shared/logging"
	"github.com/anomalyco/atlas-core/services/shared/model"
)

func classifyMissingUpdate(ctx context.Context, pool *pgxpool.Pool, log *logging.Logger, component, table, idColumn, resourceID string) error {
	query := fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s WHERE %s = $1)`, table, idColumn)
	var exists bool
	if err := pool.QueryRow(ctx, query, resourceID).Scan(&exists); err != nil {
		log.ErrorContext(ctx, component, "classify update miss failed", logging.String(idColumn, resourceID), logging.ErrorField(err))
		return fmt.Errorf("classify update miss: %w", err)
	}
	if exists {
		return model.ErrVersionConflict
	}
	return model.ErrNotFound
}
