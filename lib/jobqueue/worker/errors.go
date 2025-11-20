package worker

// PermanentError signals that the operation should not be retried.
type PermanentError struct {
	Err error
}

// Permanent wraps the given err in a *PermanentError.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &PermanentError{
		Err: err,
	}
}

// Error returns a string representation of the Permanent error.
func (e *PermanentError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the wrapped error.
func (e *PermanentError) Unwrap() error {
	return e.Err
}
