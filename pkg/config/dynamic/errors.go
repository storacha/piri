package dynamic

import (
	"fmt"

	"github.com/storacha/piri/pkg/config"
)

// ParseError indicates a value could not be parsed to the expected type.
type ParseError struct {
	Value    any
	Expected string
	Cause    error
}

func (e *ParseError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("cannot parse %v: expected %s: %v", e.Value, e.Expected, e.Cause)
	}
	return fmt.Sprintf("cannot parse %v: expected %s", e.Value, e.Expected)
}

func (e *ParseError) Unwrap() error { return e.Cause }

// TypeError indicates a type mismatch between expected and actual value.
type TypeError struct {
	Expected string
	Got      string
}

func (e *TypeError) Error() string {
	return fmt.Sprintf("type mismatch: expected %s, got %s", e.Expected, e.Got)
}

// RangeError indicates a value is outside the allowed range.
type RangeError[T any] struct {
	Value T
	Min   T
	Max   T
}

func (e *RangeError[T]) Error() string {
	return fmt.Sprintf("value %v outside valid range [%v, %v]", e.Value, e.Min, e.Max)
}

// UnknownKeyError indicates an unrecognized configuration key.
type UnknownKeyError struct {
	Key string
}

func (e *UnknownKeyError) Error() string {
	return fmt.Sprintf("unknown configuration key: %s", e.Key)
}

// ValidationError wraps a validation failure with the config key context.
type ValidationError struct {
	Key   config.Key
	Cause error
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid value for '%s': %v", e.Key, e.Cause)
}

func (e *ValidationError) Unwrap() error { return e.Cause }

// PersistError indicates a failure to persist configuration to file.
type PersistError struct {
	Cause error
}

func (e *PersistError) Error() string {
	return fmt.Sprintf("failed to persist configuration: %v", e.Cause)
}

func (e *PersistError) Unwrap() error { return e.Cause }
