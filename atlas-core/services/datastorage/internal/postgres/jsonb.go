package postgres

import (
	"encoding/json"
	"fmt"
)

func jsonbParam(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "{}", nil
	}
	if !json.Valid(raw) {
		return "", fmt.Errorf("invalid JSON payload")
	}
	return string(raw), nil
}
