package dynamic

import (
	"fmt"
	"strconv"
	"time"
)

// ConfigSchema handles parsing raw JSON values and validating them.
type ConfigSchema interface {
	// ParseAndValidate converts a raw value (from JSON) to a typed value
	// and validates it against constraints.
	// Returns the typed value or a descriptive error.
	ParseAndValidate(raw any) (any, error)

	// TypeDescription returns human-readable type info for error messages.
	TypeDescription() string
}

// DurationSchema parses and validates duration values.
// Accepts string values like "30s", "5m", "1h" or time.Duration directly.
type DurationSchema struct {
	Min time.Duration
	Max time.Duration
}

func (s DurationSchema) TypeDescription() string {
	return fmt.Sprintf("duration string (e.g., '30s', '5m'), range [%s, %s]", s.Min, s.Max)
}

func (s DurationSchema) ParseAndValidate(raw any) (any, error) {
	var d time.Duration

	switch v := raw.(type) {
	case string:
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return nil, &ParseError{
				Value:    v,
				Expected: "duration string (e.g., '30s', '5m')",
				Cause:    err,
			}
		}
		d = parsed
	case time.Duration:
		// Already typed (e.g., from internal code path)
		d = v
	default:
		return nil, &TypeError{
			Expected: "duration string",
			Got:      fmt.Sprintf("%T", raw),
		}
	}

	if s.Min > 0 && d < s.Min {
		return nil, &RangeError[time.Duration]{Value: d, Min: s.Min, Max: s.Max}
	}
	if s.Max > 0 && d > s.Max {
		return nil, &RangeError[time.Duration]{Value: d, Min: s.Min, Max: s.Max}
	}

	return d, nil
}

// IntSchema parses and validates integer values.
// Accepts int, int64, float64 (from JSON), or string representations.
type IntSchema struct {
	Min int
	Max int
}

func (s IntSchema) TypeDescription() string {
	return fmt.Sprintf("integer, range [%d, %d]", s.Min, s.Max)
}

func (s IntSchema) ParseAndValidate(raw any) (any, error) {
	var i int

	switch v := raw.(type) {
	case int:
		i = v
	case int64:
		i = int(v)
	case float64:
		// JSON unmarshals numbers as float64
		if v != float64(int(v)) {
			return nil, &ParseError{
				Value:    v,
				Expected: "integer (got floating point)",
			}
		}
		i = int(v)
	case string:
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return nil, &ParseError{Value: v, Expected: "integer", Cause: err}
		}
		i = parsed
	default:
		return nil, &TypeError{
			Expected: "integer",
			Got:      fmt.Sprintf("%T", raw),
		}
	}

	if i < s.Min {
		return nil, &RangeError[int]{Value: i, Min: s.Min, Max: s.Max}
	}
	if i > s.Max {
		return nil, &RangeError[int]{Value: i, Min: s.Min, Max: s.Max}
	}

	return i, nil
}

// UintSchema parses and validates unsigned integer values.
// Accepts uint, int, int64, float64 (from JSON), or string representations.
type UintSchema struct {
	Min uint
	Max uint
}

func (s UintSchema) TypeDescription() string {
	return fmt.Sprintf("unsigned integer, range [%d, %d]", s.Min, s.Max)
}

func (s UintSchema) ParseAndValidate(raw any) (any, error) {
	var u uint

	switch v := raw.(type) {
	case uint:
		u = v
	case int:
		if v < 0 {
			return nil, &ParseError{
				Value:    v,
				Expected: "unsigned integer (got negative value)",
			}
		}
		u = uint(v)
	case int64:
		if v < 0 {
			return nil, &ParseError{
				Value:    v,
				Expected: "unsigned integer (got negative value)",
			}
		}
		u = uint(v)
	case float64:
		// JSON unmarshals numbers as float64
		if v != float64(int(v)) {
			return nil, &ParseError{
				Value:    v,
				Expected: "unsigned integer (got floating point)",
			}
		}
		if v < 0 {
			return nil, &ParseError{
				Value:    v,
				Expected: "unsigned integer (got negative value)",
			}
		}
		u = uint(v)
	case string:
		parsed, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, &ParseError{Value: v, Expected: "unsigned integer", Cause: err}
		}
		u = uint(parsed)
	default:
		return nil, &TypeError{
			Expected: "unsigned integer",
			Got:      fmt.Sprintf("%T", raw),
		}
	}

	if u < s.Min {
		return nil, &RangeError[uint]{Value: u, Min: s.Min, Max: s.Max}
	}
	if u > s.Max {
		return nil, &RangeError[uint]{Value: u, Min: s.Min, Max: s.Max}
	}

	return u, nil
}
