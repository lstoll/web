package requestid

import (
	"net/http"
)

// Handler wraps a http.Handler, ensuring that a request ID exists on
// the context downstream. If trustIncomingHeader is true, the request ID from
// the incoming request header will be used if it exists.
func Handler(trustIncomingHeader bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requestID string
		if id, ok := FromContext(r.Context()); ok {
			requestID = id
		} else if trustIncomingHeader && r.Header.Get(RequestIDHeader) != "" {
			requestID = r.Header.Get(RequestIDHeader)
		} else {
			requestID = newRequestID()
		}

		r = r.WithContext(ContextWithRequestID(r.Context(), requestID))

		next.ServeHTTP(w, r)
	})
}
