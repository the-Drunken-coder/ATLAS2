package postgres

import (
	"testing"
	"time"
)

func TestTrimPage_RejectsNonPositivePageSize(t *testing.T) {
	items := []string{"a", "b", "c"}
	_, _, err := trimPage(items, 0, nil, func(s string) time.Time { return time.Now() }, func(s string) string { return s })
	if err == nil {
		t.Fatal("expected error for page_size 0")
	}
}
