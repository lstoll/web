package requestid

import (
	"net/http"

	"lds.li/web/internal"
)

// Middleware is a middleware that ensures a request ID is present on the
// context. If a request ID is found in one of the trusted headers, it will be
// used. If not, a new request ID will be generated.
type Middleware struct {
	// TrustedHeaders is a list of headers that are trusted to contain the
	// request ID. If a request ID is found in one of these headers, it will be
	// used instead of generating a new one. It is processed in order, the first
	// matching header is used.
	TrustedHeaders []string
}

// Handler wraps a http.Handler, ensuring that a request ID exists on the
// context downstream.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requestID string
		if _, ok := FromContext(r.Context()); ok {
			// already in context, continue
			next.ServeHTTP(w, r)
			return
		}
		for _, hdr := range m.TrustedHeaders {
			if id := r.Header.Get(hdr); id != "" {
				requestID = id
				break
			}
		}
		if requestID == "" {
			requestID = internal.NewUUIDV4().String()
		}
		r = r.WithContext(ContextWithRequestID(r.Context(), requestID))
		next.ServeHTTP(w, r)
	})
}
