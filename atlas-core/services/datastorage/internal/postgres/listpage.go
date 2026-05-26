package postgres

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/listcursor"
	"github.com/anomalyco/atlas-core/services/shared/model"
)

// appendListPagination adds strict-sync watermark and keyset predicates for
// updated_at DESC, id ASC ordering.
func appendListPagination(pageToken string, strictSnapshot bool, idColumn string, argIdx int, conditions []string, args []any) ([]string, []any, int, *time.Time, error) {
	var syncWatermark *time.Time

	if pageToken == "" {
		if strictSnapshot {
			now := time.Now().UTC()
			syncWatermark = &now
			conditions = append(conditions, fmt.Sprintf("updated_at <= $%d", argIdx))
			args = append(args, now)
			argIdx++
		}
		return conditions, args, argIdx, syncWatermark, nil
	}

	cur, err := listcursor.DecodePage(pageToken)
	if err != nil {
		return nil, nil, 0, nil, err
	}
	if cur.SyncWatermark != nil {
		syncWatermark = cur.SyncWatermark
		conditions = append(conditions, fmt.Sprintf("updated_at <= $%d", argIdx))
		args = append(args, *cur.SyncWatermark)
		argIdx++
	} else if strictSnapshot {
		return nil, nil, 0, nil, model.NewFieldError(
			"INVALID_INPUT",
			"strict_snapshot continuation requires a snapshot page_token",
			"page_token",
		)
	}

	conditions = append(conditions, fmt.Sprintf(
		"(updated_at < $%d OR (updated_at = $%d AND %s > $%d))",
		argIdx, argIdx+1, idColumn, argIdx+2,
	))
	args = append(args, cur.CursorAt, cur.CursorAt, cur.CursorID)
	return conditions, args, argIdx + 3, syncWatermark, nil
}

func listOrderLimit(pageSize int, idColumn string) string {
	return fmt.Sprintf(" ORDER BY updated_at DESC, %s ASC LIMIT %d", idColumn, pageSize+1)
}

func trimPage[T any](items []T, pageSize int, syncWatermark *time.Time, updatedAt func(T) time.Time, id func(T) string) ([]T, string, error) {
	if pageSize <= 0 {
		return nil, "", fmt.Errorf("invalid page_size: %d", pageSize)
	}
	if len(items) <= pageSize {
		return items, "", nil
	}
	last := items[pageSize-1]
	tok, err := listcursor.EncodePage(updatedAt(last), id(last), syncWatermark)
	if err != nil {
		return nil, "", err
	}
	return items[:pageSize], tok, nil
}

func versionFromUpdateClassification(classification string, newVersion sql.NullInt64) (int, error) {
	switch classification {
	case "updated":
		if !newVersion.Valid {
			return 0, fmt.Errorf("updated row missing new version")
		}
		return int(newVersion.Int64), nil
	case "conflict":
		return 0, model.ErrVersionConflict
	case "not_found":
		return 0, model.ErrNotFound
	default:
		return 0, fmt.Errorf("unexpected classification: %s", classification)
	}
}
