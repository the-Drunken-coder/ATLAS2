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
	UpdatedAt string `json:"updated_at"`
	ID        string `json:"id"`
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
	p := payload{
		UpdatedAt: updatedAt.UTC().Format(time.RFC3339Nano),
		ID:        id,
	}
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Decode parses a page_token. Empty token is invalid here — callers must skip
// pagination predicates when the client sends no token.
func Decode(token string) (updatedAt time.Time, id string, err error) {
	if token == "" {
		return time.Time{}, "", model.NewFieldError("INVALID_INPUT", "page_token is empty", "page_token")
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return time.Time{}, "", model.NewFieldError("INVALID_INPUT", "malformed page_token", "page_token")
	}
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return time.Time{}, "", model.NewFieldError("INVALID_INPUT", "malformed page_token", "page_token")
	}
	if p.UpdatedAt == "" || p.ID == "" {
		return time.Time{}, "", model.NewFieldError("INVALID_INPUT", "malformed page_token", "page_token")
	}
	if !idPattern.MatchString(p.ID) {
		return time.Time{}, "", model.NewFieldError("INVALID_INPUT", "malformed page_token", "page_token")
	}
	ts, err := time.Parse(time.RFC3339Nano, p.UpdatedAt)
	if err != nil {
		return time.Time{}, "", model.NewFieldError("INVALID_INPUT", "malformed page_token", "page_token")
	}
	return ts.UTC(), p.ID, nil
}
