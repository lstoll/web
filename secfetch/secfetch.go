// Package secfetch provides middleware for protecting web applications using Fetch Metadata headers.
//
// This package helps protect against CSRF and other cross-site attacks by checking
// the Sec-Fetch-* headers sent by modern browsers. These headers provide information
// about the context of the request, such as the initiator, mode, and destination.
//
// The basic middleware denies cross-site requests by default and can be configured with
// options to allow specific types of cross-site requests.
package secfetch

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// Errors
var (
	// ErrMissingSecFetchHeaders is returned when required Sec-Fetch headers are missing
	ErrMissingSecFetchHeaders = errors.New("missing required Sec-Fetch headers")

	// ErrCrossSiteRequest is returned when a cross-site request is rejected
	ErrCrossSiteRequest = errors.New("cross-site request rejected")

	// ErrInvalidMode is returned when the Sec-Fetch-Mode is not in the allowed list
	ErrInvalidMode = errors.New("invalid request mode")

	// ErrInvalidDest is returned when the Sec-Fetch-Dest is not in the allowed list
	ErrInvalidDest = errors.New("invalid request destination")
)

// Option defines the interface for configuration options for the secfetch middleware
type Option interface{}

// AllowCrossSiteNavigation is an option that allows cross-site navigation requests.
// This is useful for allowing users to follow links from other sites to your site.
// Only requests with Sec-Fetch-Mode: navigate will be allowed.
type AllowCrossSiteNavigation struct{}

// AllowCrossSiteAPI is an option that allows cross-site API requests.
// This is useful for public APIs that need to be accessible from other domains.
// Only requests with Sec-Fetch-Mode: cors or no-cors will be allowed.
type AllowCrossSiteAPI struct{}

// AllowedModes specifies which Sec-Fetch-Mode values are acceptable.
// Common values include: "navigate", "same-origin", "cors", "no-cors", "websocket"
type AllowedModes []string

// AllowedDests specifies which Sec-Fetch-Dest values are acceptable.
// Common values include: "document", "empty", "image", "style", "script", "frame"
type AllowedDests []string

// DefaultOptions provides sensible default security options
var DefaultOptions = []Option{
	AllowedModes{"navigate", "same-origin"},
	AllowedDests{"document", "empty"},
}

// Protect creates middleware that enforces Sec-Fetch header restrictions.
// It wraps the provided handler with checks for Sec-Fetch headers and
// rejects requests that don't meet the security criteria.
//
// By default, it blocks cross-site requests and only allows specific modes and destinations.
// Use the provided options to customize the behavior.
//
// Example:
//
//	handler := secfetch.Protect(
//		myHandler,
//		secfetch.AllowCrossSiteNavigation{},
//		secfetch.WithAllowedModes("navigate", "same-origin", "cors"),
//	)
func Protect(h http.Handler, opts ...Option) http.Handler {
	// Default allowed modes and destinations
	allowedModes := []string{"navigate", "same-origin"}
	allowedDests := []string{"document", "empty"}
	allowCrossSiteNav := false
	allowCrossSiteAPI := false

	// Process options
	for _, opt := range opts {
		switch o := opt.(type) {
		case AllowedModes:
			allowedModes = o
		case AllowedDests:
			allowedDests = o
		case AllowCrossSiteNavigation:
			allowCrossSiteNav = true
		case AllowCrossSiteAPI:
			allowCrossSiteAPI = true
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip preflight requests
		if r.Method == http.MethodOptions {
			h.ServeHTTP(w, r)
			return
		}

		secFetchSite := r.Header.Get("Sec-Fetch-Site")
		secFetchMode := r.Header.Get("Sec-Fetch-Mode")
		secFetchDest := r.Header.Get("Sec-Fetch-Dest")

		// Check if headers are present (modern browsers)
		if secFetchSite == "" || secFetchMode == "" || secFetchDest == "" {
			if r.Method != http.MethodGet {
				http.Error(w, "Missing required security headers", http.StatusForbidden)
				return
			}
			// For GET requests without headers, we'll let it pass but with caution
			// This is to handle older browsers, but the assumption is we're not supporting them
			h.ServeHTTP(w, r)
			return
		}

		// Validate Sec-Fetch-Site
		if secFetchSite == "cross-site" {
			// Allow cross-site navigation if explicitly enabled
			if allowCrossSiteNav && secFetchMode == "navigate" {
				h.ServeHTTP(w, r)
				return
			}

			// Allow cross-site API if explicitly enabled
			if allowCrossSiteAPI && (secFetchMode == "cors" || secFetchMode == "no-cors") {
				h.ServeHTTP(w, r)
				return
			}

			// Otherwise reject cross-site
			http.Error(w, "Cross-site request rejected", http.StatusForbidden)
			return
		}

		// Validate Sec-Fetch-Mode
		if !slices.Contains(allowedModes, secFetchMode) {
			http.Error(w, fmt.Sprintf("Invalid request mode: %s", secFetchMode), http.StatusForbidden)
			return
		}

		// Validate Sec-Fetch-Dest
		if !slices.Contains(allowedDests, secFetchDest) {
			http.Error(w, fmt.Sprintf("Invalid request destination: %s", secFetchDest), http.StatusForbidden)
			return
		}

		// For POST/PUT requests, be extra strict about form submissions
		if (r.Method == http.MethodPost || r.Method == http.MethodPut) &&
			strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
			// Form submissions should have Sec-Fetch-User: ?1 for user-initiated actions
			secFetchUser := r.Header.Get("Sec-Fetch-User")
			if secFetchUser != "?1" && secFetchMode != "cors" {
				http.Error(w, "Form submission not user-initiated", http.StatusForbidden)
				return
			}
		}

		// All checks passed, continue to the handler
		h.ServeHTTP(w, r)
	})
}

// WithAllowedModes creates an option to specify allowed request modes.
// It controls which values of the Sec-Fetch-Mode header are acceptable.
//
// Example:
//
//	secfetch.WithAllowedModes("navigate", "same-origin", "cors")
func WithAllowedModes(modes ...string) AllowedModes {
	return modes
}

// WithAllowedDests creates an option to specify allowed request destinations.
// It controls which values of the Sec-Fetch-Dest header are acceptable.
//
// Example:
//
//	secfetch.WithAllowedDests("document", "empty", "image")
func WithAllowedDests(dests ...string) AllowedDests {
	return dests
}
