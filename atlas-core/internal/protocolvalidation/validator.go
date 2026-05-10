package protocolvalidation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/anomalyco/atlas-core/internal/core/model"
)

type Operation string

const (
	OperationCreate Operation = "create"
	OperationUpdate Operation = "update"
	OperationUpsert Operation = "upsert"
)

type JSONValidator interface {
	NormalizeEntity(context.Context, *model.Entity, Operation) error
	NormalizeObject(context.Context, *model.Object, Operation) error
	NormalizeTask(context.Context, *model.Task, Operation) error
	NormalizeObservation(context.Context, *model.Observation, Operation) error
}

type Runner struct {
	nodeBin       string
	validatorPath string
	timeout       time.Duration
}

type cliResponse struct {
	OK         bool        `json:"ok"`
	Normalized string      `json:"normalized"`
	Errors     []Violation `json:"errors"`
}

func NewRunner() *Runner {
	return &Runner{timeout: 5 * time.Second}
}

func (r *Runner) NormalizeEntity(ctx context.Context, entity *model.Entity, op Operation) error {
	response, err := r.validate(ctx, "entity", entity.JSON, []string{"--entity-type", string(entity.Type), "--operation", string(op)})
	if err != nil {
		return err
	}
	entity.JSON = bytes.TrimSpace([]byte(response.Normalized))
	return nil
}

func (r *Runner) NormalizeObject(ctx context.Context, obj *model.Object, op Operation) error {
	response, err := r.validate(ctx, "object", obj.JSON, []string{"--object-type", string(obj.Type), "--object-id", obj.ObjectID, "--operation", string(op)})
	if err != nil {
		return err
	}
	obj.JSON = bytes.TrimSpace([]byte(response.Normalized))
	return nil
}

func (r *Runner) NormalizeTask(ctx context.Context, task *model.Task, op Operation) error {
	response, err := r.validate(ctx, "task", task.JSON, []string{"--operation", string(op)})
	if err != nil {
		return err
	}
	task.JSON = bytes.TrimSpace([]byte(response.Normalized))
	return nil
}

func (r *Runner) NormalizeObservation(ctx context.Context, obs *model.Observation, op Operation) error {
	response, err := r.validate(ctx, "observation", obs.JSON, []string{"--operation", string(op)})
	if err != nil {
		return err
	}
	obs.JSON = bytes.TrimSpace([]byte(response.Normalized))
	return nil
}

func (r *Runner) validate(ctx context.Context, schema string, raw []byte, extraArgs []string) (*cliResponse, error) {
	validatorPath, err := r.resolveValidatorPath()
	if err != nil {
		return nil, err
	}
	nodeBin := r.nodeBin
	if nodeBin == "" {
		nodeBin = os.Getenv("ATLAS_PROTOCOL_NODE_BIN")
		if nodeBin == "" {
			nodeBin = "node"
		}
	}
	if raw == nil {
		raw = []byte("{}")
	}
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	args := append([]string{validatorPath, "--schema", schema}, extraArgs...)
	cmd := exec.CommandContext(ctx, nodeBin, args...)
	cmd.Stdin = bytes.NewReader(raw)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	var response cliResponse
	if decodeErr := json.Unmarshal(stdout.Bytes(), &response); decodeErr == nil {
		if !response.OK {
			return nil, &ValidationError{Violations: response.Errors}
		}
		if runErr == nil {
			return &response, nil
		}
	}

	if ctx.Err() == context.DeadlineExceeded {
		return nil, model.NewCoreError("PROTOCOL_VALIDATION_ERROR", "atlas-protocol validator timed out")
	}
	if ctx.Err() == context.Canceled {
		return nil, model.NewCoreError("PROTOCOL_VALIDATION_ERROR", "atlas-protocol validator canceled")
	}
	message := strings.TrimSpace(stderr.String())
	if message == "" && runErr != nil {
		message = runErr.Error()
	}
	return nil, model.NewCoreError("PROTOCOL_VALIDATION_ERROR", fmt.Sprintf("atlas-protocol validator failed: %s", message))
}

func (r *Runner) resolveValidatorPath() (string, error) {
	if r.validatorPath != "" {
		return r.validatorPath, nil
	}
	if configured := os.Getenv("ATLAS_PROTOCOL_VALIDATOR_PATH"); configured != "" {
		return configured, nil
	}
	for _, start := range []string{mustGetwd(), executableDir()} {
		if start == "" {
			continue
		}
		if found := searchUpward(start, filepath.Join("protocol", "atlas-protocol-validator.mjs")); found != "" {
			return found, nil
		}
	}
	return "", model.NewCoreError("PROTOCOL_VALIDATION_ERROR", "atlas-protocol validator artifact not found; run `python3 atlas.py protocol-sync`")
}

func mustGetwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}

func executableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}

func searchUpward(start, suffix string) string {
	dir := start
	for {
		candidate := filepath.Join(dir, suffix)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
