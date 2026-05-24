package listcursor

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/anomalyco/atlas-core/services/shared/model"
)

func TestNormalizePageSize(t *testing.T) {
	n, err := NormalizePageSize(0)
	if err != nil || n != DefaultPageSize {
		t.Fatalf("zero: got %d err=%v", n, err)
	}
	n, err = NormalizePageSize(50)
	if err != nil || n != 50 {
		t.Fatalf("50: got %d err=%v", n, err)
	}
	if _, err := NormalizePageSize(-1); err == nil {
		t.Fatal("expected error for negative")
	}
	if _, err := NormalizePageSize(501); err == nil {
		t.Fatal("expected error for oversized")
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	ts := time.Date(2026, 5, 10, 12, 30, 45, 123456789, time.UTC)
	tok, err := Encode(ts, "ent_abc")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	gotTs, id, err := Decode(tok)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !gotTs.Equal(ts) || id != "ent_abc" {
		t.Fatalf("round trip: got %v %q", gotTs, id)
	}
}

func TestEncodePageSnapshotWatermarkRoundTrip(t *testing.T) {
	ts := time.Date(2026, 5, 10, 12, 30, 45, 0, time.UTC)
	wm := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	tok, err := EncodePage(ts, "ent_abc", &wm)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	cur, err := DecodePage(tok)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !cur.CursorAt.Equal(ts) || cur.CursorID != "ent_abc" {
		t.Fatalf("cursor: got %v %q", cur.CursorAt, cur.CursorID)
	}
	if cur.SyncWatermark == nil || !cur.SyncWatermark.Equal(wm) {
		t.Fatalf("watermark: got %v", cur.SyncWatermark)
	}
}

func TestDecodeMalformed(t *testing.T) {
	if _, _, err := Decode("not-base64!!!"); err == nil {
		t.Fatal("expected error")
	}
	if _, _, err := Decode("e30"); err == nil { // base64 of "{}"
		t.Fatal("expected error for empty json fields")
	}
	badID := base64.RawURLEncoding.EncodeToString([]byte(`{"updated_at":"2026-05-10T12:30:45Z","id":"../bad"}`))
	if _, _, err := Decode(badID); err == nil {
		t.Fatal("expected error for invalid id shape")
	}
}

func TestDecodeInvalidFieldError(t *testing.T) {
	_, _, err := Decode("!!!")
	var fe *model.FieldError
	if err == nil || !errors.As(err, &fe) || fe.Code != "INVALID_INPUT" || fe.Field != "page_token" {
		t.Fatalf("expected INVALID_INPUT page_token FieldError, got %v", err)
	}
}
