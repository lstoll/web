// Package cors provides a middleware to handle CORS pre-flight requests.
package cors

import "net/http"

// DenyPreflight handles CORS pre-flight requests by denying them in all cases.
func DenyPreflight(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
