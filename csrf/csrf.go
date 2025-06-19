package csrf

import (
	"context"
	"net/http"
)

type skipContextKey struct{}

// Skip marks the request to be skipped for CSRF protection.
func Skip(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), skipContextKey{}, true))
}

type Handler struct {
	*http.CrossOriginProtection
}

func NewHandler(csrf *http.CrossOriginProtection) *Handler {
	return &Handler{CrossOriginProtection: csrf}
}

// OptHandler wraps the handler with a CSRF protection handler, honouring the
// skip option.
func (hh *Handler) Handler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Context().Value(skipContextKey{}).(bool); ok {
			h.ServeHTTP(w, r)
			return
		}
		hh.CrossOriginProtection.Handler(h).ServeHTTP(w, r)
	})
}
