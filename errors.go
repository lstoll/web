package web

import "fmt"

// ErrForbidden is a 403 error
type ErrForbidden struct {
	error
}

func (e *ErrForbidden) Unwrap() error {
	return e.error
}

func ForbiddenErrf(format string, args ...any) error {
	return &ErrForbidden{
		error: fmt.Errorf(format, args...),
	}
}

// ErrBadRequest is a 401 error
type ErrBadRequest struct {
	error
}

func (e *ErrBadRequest) Unwrap() error {
	return e.error
}

func BadRequestErrf(format string, args ...any) error {
	return &ErrBadRequest{
		error: fmt.Errorf(format, args...),
	}
}
