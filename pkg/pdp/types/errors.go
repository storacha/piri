package types

import (
	"fmt"
)

// Kind represents the category of error
type Kind int

const (
	KindOther Kind = iota
	KindNotFound
	KindInvalidInput
	KindUnauthorized
	KindInternal
)

// Error represents a service error with a kind
type Error struct {
	kind Kind
	msg  string
	err  error
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.err != nil {
		return fmt.Sprintf("%s: %s", e.msg, e.err.Error())
	}
	return e.msg
}

// Kind returns the error kind
func (e *Error) Kind() Kind {
	return e.kind
}

// Unwrap returns the wrapped error
func (e *Error) Unwrap() error {
	return e.err
}

// NewError creates a new error with the given kind and message
func NewError(kind Kind, msg string) *Error {
	return &Error{kind: kind, msg: msg}
}

func NewErrorf(kind Kind, msg string, args ...interface{}) *Error {
	return &Error{kind: kind, msg: fmt.Sprintf(msg, args...)}
}

// WrapError wraps an existing error with a kind and message
func WrapError(kind Kind, msg string, err error) *Error {
	return &Error{kind: kind, msg: msg, err: err}
}
