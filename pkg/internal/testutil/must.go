package testutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Must takes return values from a function and returns the non-error one. If
// the error value is non-nil then it fails the test
func Must[T any](val T, err error) func(testing.TB) T {
	return func(t testing.TB) T {
		require.NoError(t, err)
		return val
	}
}

// Must2 takes return values from a 3 return function and returns the non-error ones. If
// the error value is non-nil then it fails the test.
func Must2[T, U any](val1 T, val2 U, err error) func(*testing.T) (T, U) {
	return func(t *testing.T) (T, U) {
		require.NoError(t, err)
		return val1, val2
	}
}
