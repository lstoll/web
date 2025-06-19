//go:build !go1.25

package csrf

import (
	"net/http"

	"filippo.io/csrf"
)

type skipContextKey struct{}

// Skip marks the request to be skipped for CSRF protection.
var Skip = csrf.UnsafeBypassRequest

type Handler struct {
	*csrf.Protection
}

func New() *Handler {
	return &Handler{Protection: csrf.New()}
}

func NewWithProtection(csrf *csrf.Protection) *Handler {
	return &Handler{Protection: csrf}
}

func (hh *Handler) Handler(h http.Handler) http.Handler {
	return hh.Protection.Handler(h)
}
