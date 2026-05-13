package protocol

import "sort"

type ResourceKind string

const (
	ResourceEntity         ResourceKind = "entity"
	ResourceTask           ResourceKind = "task"
	ResourceObservation    ResourceKind = "observation"
	ResourceObject         ResourceKind = "object"
	ResourceCommandCatalog ResourceKind = "commandCatalog"
	ResourceChangeEvent    ResourceKind = "changeEvent"
	ResourceCustomSection  ResourceKind = "customSection"
)

type ValidationIssue struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ValidateOption func(*validateOptions)

type validateOptions struct {
	variant string
}

func WithVariant(variant string) ValidateOption {
	return func(o *validateOptions) { o.variant = variant }
}

func New() (*Validator, error) {
	v := &Validator{useEmbedded: true}
	if err := v.ensureCompiler(); err != nil {
		return nil, err
	}
	return v, nil
}

func NewWithProtocolRoot(root string) (*Validator, error) {
	v := &Validator{protocolRoot: root, useEmbedded: false}
	if err := v.ensureCompiler(); err != nil {
		return nil, err
	}
	return v, nil
}

func NormalizeValidationIssues(issues []ValidationIssue) []ValidationIssue {
	out := append([]ValidationIssue(nil), issues...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Field != out[j].Field {
			return out[i].Field < out[j].Field
		}
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		return out[i].Message < out[j].Message
	})
	return out
}
