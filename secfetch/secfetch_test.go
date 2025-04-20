package secfetch

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProtect(t *testing.T) {
	// Create a simple test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("success"))
	})

	tests := []struct {
		name           string
		method         string
		headers        map[string]string
		options        []Option
		expectedStatus int
	}{
		{
			name:   "same-origin request should pass",
			method: "GET",
			headers: map[string]string{
				"Sec-Fetch-Site": "same-origin",
				"Sec-Fetch-Mode": "navigate",
				"Sec-Fetch-Dest": "document",
			},
			options:        nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:   "same-site request should pass",
			method: "GET",
			headers: map[string]string{
				"Sec-Fetch-Site": "same-site",
				"Sec-Fetch-Mode": "navigate",
				"Sec-Fetch-Dest": "document",
			},
			options:        nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:   "cross-site request should fail",
			method: "GET",
			headers: map[string]string{
				"Sec-Fetch-Site": "cross-site",
				"Sec-Fetch-Mode": "navigate",
				"Sec-Fetch-Dest": "document",
			},
			options:        nil,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:   "cross-site navigation should pass with option",
			method: "GET",
			headers: map[string]string{
				"Sec-Fetch-Site": "cross-site",
				"Sec-Fetch-Mode": "navigate",
				"Sec-Fetch-Dest": "document",
			},
			options:        []Option{AllowCrossSiteNavigation{}},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "cross-site API request should pass with option",
			method: "GET",
			headers: map[string]string{
				"Sec-Fetch-Site": "cross-site",
				"Sec-Fetch-Mode": "cors",
				"Sec-Fetch-Dest": "empty",
			},
			options:        []Option{AllowCrossSiteAPI{}},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "invalid mode should fail",
			method: "GET",
			headers: map[string]string{
				"Sec-Fetch-Site": "same-origin",
				"Sec-Fetch-Mode": "websocket", // Not in default allowed modes
				"Sec-Fetch-Dest": "document",
			},
			options:        nil,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:   "custom allowed mode should pass",
			method: "GET",
			headers: map[string]string{
				"Sec-Fetch-Site": "same-origin",
				"Sec-Fetch-Mode": "websocket",
				"Sec-Fetch-Dest": "document",
			},
			options:        []Option{WithAllowedModes("navigate", "same-origin", "websocket")},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "invalid dest should fail",
			method: "GET",
			headers: map[string]string{
				"Sec-Fetch-Site": "same-origin",
				"Sec-Fetch-Mode": "navigate",
				"Sec-Fetch-Dest": "iframe", // Not in default allowed dests
			},
			options:        nil,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:   "custom allowed dest should pass",
			method: "GET",
			headers: map[string]string{
				"Sec-Fetch-Site": "same-origin",
				"Sec-Fetch-Mode": "navigate",
				"Sec-Fetch-Dest": "iframe",
			},
			options:        []Option{WithAllowedDests("document", "empty", "iframe")},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "POST without Sec-Fetch headers should fail",
			method: "POST",
			headers: map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
			},
			options:        nil,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:   "POST with form needs user interaction",
			method: "POST",
			headers: map[string]string{
				"Sec-Fetch-Site": "same-origin",
				"Sec-Fetch-Mode": "navigate",
				"Sec-Fetch-Dest": "document",
				"Content-Type":   "application/x-www-form-urlencoded",
				// Missing Sec-Fetch-User
			},
			options:        nil,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:   "POST with user interaction should pass",
			method: "POST",
			headers: map[string]string{
				"Sec-Fetch-Site": "same-origin",
				"Sec-Fetch-Mode": "navigate",
				"Sec-Fetch-Dest": "document",
				"Content-Type":   "application/x-www-form-urlencoded",
				"Sec-Fetch-User": "?1",
			},
			options:        nil,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the middleware with options
			handler := Protect(testHandler, tt.options...)

			// Create request
			req := httptest.NewRequest(tt.method, "http://example.com", nil)

			// Add headers
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			// Send request
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			// Check status
			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestWithOptions(t *testing.T) {
	modes := WithAllowedModes("a", "b", "c")
	if len(modes) != 3 || modes[0] != "a" || modes[1] != "b" || modes[2] != "c" {
		t.Errorf("WithAllowedModes failed to create proper modes: %v", modes)
	}

	dests := WithAllowedDests("x", "y", "z")
	if len(dests) != 3 || dests[0] != "x" || dests[1] != "y" || dests[2] != "z" {
		t.Errorf("WithAllowedDests failed to create proper destinations: %v", dests)
	}
}
