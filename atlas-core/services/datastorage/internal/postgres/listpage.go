package postgres

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/listcursor"
	"github.com/anomalyco/atlas-core/services/shared/model"
)

// appendKeysetCursor adds keyset pagination predicates for updated_at DESC, id ASC ordering.
func appendKeysetCursor(pageToken, idColumn string, argIdx int, conditions []string, args []any) ([]string, []any, int, error) {
	if pageToken == "" {
		return conditions, args, argIdx, nil
	}
	cursorAt, cursorID, err := listcursor.Decode(pageToken)
	if err != nil {
		return nil, nil, 0, err
	}
	conditions = append(conditions, fmt.Sprintf(
		"(updated_at < $%d OR (updated_at = $%d AND %s > $%d))",
		argIdx, argIdx+1, idColumn, argIdx+2,
	))
	args = append(args, cursorAt, cursorAt, cursorID)
	return conditions, args, argIdx + 3, nil
}

func listOrderLimit(pageSize int, idColumn string) string {
	return fmt.Sprintf(" ORDER BY updated_at DESC, %s ASC LIMIT %d", idColumn, pageSize+1)
}

func trimPage[T any](items []T, pageSize int, updatedAt func(T) time.Time, id func(T) string) ([]T, string, error) {
	if len(items) <= pageSize {
		return items, "", nil
	}
	last := items[pageSize-1]
	tok, err := listcursor.Encode(updatedAt(last), id(last))
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
