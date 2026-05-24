// Package listcursor encodes cursor tokens for stable keyset pagination
// (updated_at DESC, primary id ASC).
package listcursor

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/model"
)

const DefaultPageSize = 100

const MaxPageSize = 500

type payload struct {
	UpdatedAt      string `json:"updated_at"`
	ID             string `json:"id"`
	SyncWatermark  string `json:"sync_watermark,omitempty"`
}

// PageCursor is pagination state encoded in page_token.
type PageCursor struct {
	CursorAt      time.Time
	CursorID      string
	SyncWatermark *time.Time
}

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// NormalizePageSize returns the effective page size or INVALID_INPUT.
func NormalizePageSize(pageSize int32) (int, error) {
	if pageSize < 0 {
		return 0, model.NewFieldError("INVALID_INPUT", "page_size must not be negative", "page_size")
	}
	if pageSize > MaxPageSize {
		return 0, model.NewFieldError("INVALID_INPUT", "page_size exceeds maximum of 500", "page_size")
	}
	if pageSize == 0 {
		return DefaultPageSize, nil
	}
	return int(pageSize), nil
}

// Encode returns a base64url(JSON) cursor from the last row of the current page.
func Encode(updatedAt time.Time, id string) (string, error) {
	return EncodePage(updatedAt, id, nil)
}

// EncodePage returns a page token, optionally including a strict-sync watermark.
func EncodePage(updatedAt time.Time, id string, syncWatermark *time.Time) (string, error) {
	p := payload{
		UpdatedAt: updatedAt.UTC().Format(time.RFC3339Nano),
		ID:        id,
	}
	if syncWatermark != nil {
		p.SyncWatermark = syncWatermark.UTC().Format(time.RFC3339Nano)
	}
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Decode parses a page_token cursor without a snapshot watermark.
func Decode(token string) (updatedAt time.Time, id string, err error) {
	cur, err := DecodePage(token)
	if err != nil {
		return time.Time{}, "", err
	}
	return cur.CursorAt, cur.CursorID, nil
}

// DecodePage parses a page_token into cursor and optional strict-sync watermark.
func DecodePage(token string) (PageCursor, error) {
	if token == "" {
		return PageCursor{}, model.NewFieldError("INVALID_INPUT", "page_token is empty", "page_token")
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return PageCursor{}, model.NewFieldError("INVALID_INPUT", "malformed page_token", "page_token")
	}
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return PageCursor{}, model.NewFieldError("INVALID_INPUT", "malformed page_token", "page_token")
	}
	if p.UpdatedAt == "" || p.ID == "" {
		return PageCursor{}, model.NewFieldError("INVALID_INPUT", "malformed page_token", "page_token")
	}
	if !idPattern.MatchString(p.ID) {
		return PageCursor{}, model.NewFieldError("INVALID_INPUT", "malformed page_token", "page_token")
	}
	ts, err := time.Parse(time.RFC3339Nano, p.UpdatedAt)
	if err != nil {
		return PageCursor{}, model.NewFieldError("INVALID_INPUT", "malformed page_token", "page_token")
	}
	out := PageCursor{
		CursorAt: ts.UTC(),
		CursorID: p.ID,
	}
	if p.SyncWatermark != "" {
		wm, err := time.Parse(time.RFC3339Nano, p.SyncWatermark)
		if err != nil {
			return PageCursor{}, model.NewFieldError("INVALID_INPUT", "malformed page_token", "page_token")
		}
		wm = wm.UTC()
		out.SyncWatermark = &wm
	}
	return out, nil
}
