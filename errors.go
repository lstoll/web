package web

import (
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
)

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

// ErrBadRequest is a 400 error
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

func DefaultErrorHandler(w http.ResponseWriter, r *http.Request, _ *template.Template, err error) {
	var forbiddenErr *ErrForbidden
	if errors.As(err, &forbiddenErr) {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	var badReqERR *ErrBadRequest
	if errors.As(err, &badReqERR) {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	slog.ErrorContext(r.Context(), "internal error in web handler", "err", err)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
