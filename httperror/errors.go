package httperror

import (
	"errors"
	"fmt"
	"net/http"
)

// HTTPError is a type that errors can implement to signal various HTTP
// statuses.
type HTTPError interface {
	error
	Code() int
}

// httpErr is the base implementation of HTTPError
type httpErr struct {
	error
	code int
}

func (e *httpErr) Code() int {
	return e.code
}

// Error returns the error message with status code information
func (e *httpErr) Error() string {
	return fmt.Sprintf("http error %d: %v", e.Code(), e.error)
}

// Newf creates a new HTTPError with the given status code and formatted error message
func Newf(code int, format string, args ...any) HTTPError {
	return &httpErr{
		error: fmt.Errorf(format, args...),
		code:  code,
	}
}

func New(code int, message string) HTTPError {
	return &httpErr{
		error: errors.New(message),
		code:  code,
	}
}

// Convenience functions for common HTTP errors
func BadRequestErrf(format string, args ...any) HTTPError {
	return Newf(http.StatusBadRequest, format, args...)
}

func ForbiddenErrf(format string, args ...any) HTTPError {
	return Newf(http.StatusForbidden, format, args...)
}

func NotFoundErrf(format string, args ...any) HTTPError {
	return Newf(http.StatusNotFound, format, args...)
}
