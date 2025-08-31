package web

import "net/http"

// BaseHeaders sets basic security headers for all requests:
// - X-Frame-Options: SAMEORIGIN
// - X-XSS-Protection: 1; mode=block
// - X-Content-Type-Options: nosniff
func BaseHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set security headers
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		h.ServeHTTP(w, r)
	})
}
