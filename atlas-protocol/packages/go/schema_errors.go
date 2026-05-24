package protocol

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

func walkValidationError(e *jsonschema.ValidationError, out *[]ValidationIssue) {
	if e == nil {
		return
	}
	if len(e.Causes) == 1 {
		if _, ok := e.ErrorKind.(*kind.Reference); ok {
			walkValidationError(e.Causes[0], out)
			return
		}
	}
	if len(e.Causes) == 0 {
		if _, ok := e.ErrorKind.(*kind.Schema); ok {
			return
		}
		*out = append(*out, mapErrorKind(e)...)
		return
	}
	for _, c := range e.Causes {
		walkValidationError(c, out)
	}
}

func mapSchemaValidateError(err error) []ValidationIssue {
	var verr *jsonschema.ValidationError
	if !errors.As(err, &verr) {
		return []ValidationIssue{{Field: "json", Code: "invalid_value", Message: err.Error()}}
	}
	var out []ValidationIssue
	walkValidationError(verr, &out)
	return out
}

func instanceTokensToField(tokens []string, child ...string) string {
	parts := []string{"json"}
	for _, seg := range tokens {
		if seg == "" {
			continue
		}
		if _, err := strconv.ParseUint(seg, 10, 64); err == nil {
			if len(parts) == 0 {
				parts = append(parts, "json")
			}
			last := parts[len(parts)-1]
			parts[len(parts)-1] = last + "[" + seg + "]"
		} else {
			parts = append(parts, seg)
		}
	}
	for _, c := range child {
		parts = append(parts, c)
	}
	return strings.Join(parts, ".")
}

func typeMismatchMessage(want []string, field string) string {
	seg := lastFieldSegment(field)
	if len(want) == 1 {
		if want[0] == "object" {
			return seg + " must be an object"
		}
		return seg + " must be a " + want[0]
	}
	return seg + " must be " + strings.Join(want, " or ")
}

func lastFieldSegment(field string) string {
	parts := strings.Split(field, ".")
	if len(parts) == 0 {
		return field
	}
	return parts[len(parts)-1]
}

func mapErrorKind(e *jsonschema.ValidationError) []ValidationIssue {
	loc := e.InstanceLocation
	switch k := e.ErrorKind.(type) {
	case *kind.Required:
		var out []ValidationIssue
		for _, m := range k.Missing {
			f := instanceTokensToField(loc, m)
			out = append(out, ValidationIssue{
				Field:   f,
				Code:    "required",
				Message: m + " is required",
			})
		}
		return out
	case *kind.AdditionalProperties:
		var out []ValidationIssue
		for _, p := range k.Properties {
			f := instanceTokensToField(loc, p)
			out = append(out, ValidationIssue{
				Field:   f,
				Code:    "unknown_field",
				Message: p + " is not allowed",
			})
		}
		return out
	case *kind.Enum:
		f := instanceTokensToField(loc)
		labels := make([]string, 0, len(k.Want))
		for _, v := range k.Want {
			labels = append(labels, fmt.Sprint(v))
		}
		return []ValidationIssue{{
			Field:   f,
			Code:    "invalid_value",
			Message: fmt.Sprintf("%s must be one of %s", lastFieldSegment(f), strings.Join(labels, ", ")),
		}}
	case *kind.Type:
		f := instanceTokensToField(loc)
		msg := typeMismatchMessage(k.Want, f)
		return []ValidationIssue{{Field: f, Code: "invalid_type", Message: msg}}
	case *kind.MinLength:
		f := instanceTokensToField(loc)
		return []ValidationIssue{{Field: f, Code: "invalid_value", Message: lastFieldSegment(f) + " must not be empty"}}
	case *kind.MinProperties:
		f := instanceTokensToField(loc)
		return []ValidationIssue{{Field: f, Code: "invalid_value", Message: lastFieldSegment(f) + " must include at least one property"}}
	case *kind.Format:
		f := instanceTokensToField(loc)
		fmtName := k.Want
		if fmtName == "" {
			fmtName = "format"
		}
		return []ValidationIssue{{Field: f, Code: "invalid_value", Message: lastFieldSegment(f) + " must match " + fmtName + " format"}}
	case *kind.Minimum, *kind.Maximum, *kind.ExclusiveMinimum, *kind.ExclusiveMaximum:
		f := instanceTokensToField(loc)
		return []ValidationIssue{{Field: f, Code: "invalid_value", Message: lastFieldSegment(f) + " is out of range"}}
	case *kind.Const:
		f := instanceTokensToField(loc)
		return []ValidationIssue{{Field: f, Code: "invalid_value", Message: lastFieldSegment(f) + " is invalid"}}
	case *kind.Group, *kind.Reference, *kind.Schema:
		return nil
	default:
		f := instanceTokensToField(loc)
		return []ValidationIssue{{Field: f, Code: "invalid_value", Message: e.Error()}}
	}
}
