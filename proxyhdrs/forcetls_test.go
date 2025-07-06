package proxyhdrs

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestForceTLS_Handle(t *testing.T) {
	tests := []struct {
		name                 string
		requestURL           string
		tls                  bool
		forwardedProtoHeader string
		forwardedProtoValue  string
		bypassPatterns       []string
		expectedStatusCode   int
		expectedLocation     string
		expectedResponseBody string
	}{
		{
			name:                 "HTTPS request should pass through",
			requestURL:           "https://example.com/test",
			tls:                  true,
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: "success",
		},
		{
			name:                 "HTTP request with forwarded proto https should pass through",
			requestURL:           "http://example.com/test",
			tls:                  false,
			forwardedProtoHeader: "X-Forwarded-Proto",
			forwardedProtoValue:  "https",
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: "success",
		},
		{
			name:                 "HTTP request with custom forwarded proto header should pass through",
			requestURL:           "http://example.com/test",
			tls:                  false,
			forwardedProtoHeader: "X-Custom-Forwarded-Proto",
			forwardedProtoValue:  "https",
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: "success",
		},
		{
			name:                 "HTTP request with forwarded proto http should redirect",
			requestURL:           "http://example.com/test",
			tls:                  false,
			forwardedProtoHeader: "X-Forwarded-Proto",
			forwardedProtoValue:  "http",
			expectedStatusCode:   http.StatusPermanentRedirect,
			expectedLocation:     "https://example.com/test",
		},
		{
			name:                 "HTTP request with bypass pattern should pass through",
			requestURL:           "http://example.com/plain",
			tls:                  false,
			bypassPatterns:       []string{"/plain"},
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: "success",
		},
		{
			name:                 "HTTP request with bypass pattern should pass through (multiple patterns)",
			requestURL:           "http://example.com/api/health",
			tls:                  false,
			bypassPatterns:       []string{"/plain", "/api/health"},
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: "success",
		},
		{
			name:               "HTTP request without bypass pattern should redirect",
			requestURL:         "http://example.com/test",
			tls:                false,
			expectedStatusCode: http.StatusPermanentRedirect,
			expectedLocation:   "https://example.com/test",
		},
		{
			name:                 "HTTP request with empty forwarded proto should redirect",
			requestURL:           "http://example.com/test",
			tls:                  false,
			forwardedProtoHeader: "X-Forwarded-Proto",
			forwardedProtoValue:  "",
			expectedStatusCode:   http.StatusPermanentRedirect,
			expectedLocation:     "https://example.com/test",
		},
		{
			name:               "HTTP request with query parameters should redirect preserving params",
			requestURL:         "http://example.com/test?param1=value1&param2=value2",
			tls:                false,
			expectedStatusCode: http.StatusPermanentRedirect,
			expectedLocation:   "https://example.com/test?param1=value1&param2=value2",
		},
		{
			name:               "HTTP request with custom port should redirect preserving port",
			requestURL:         "http://example.com:8080/test",
			tls:                false,
			expectedStatusCode: http.StatusPermanentRedirect,
			expectedLocation:   "https://example.com:8080/test",
		},
		{
			name:               "HTTP request with bypass pattern but different path should redirect",
			requestURL:         "http://example.com/other",
			tls:                false,
			bypassPatterns:     []string{"/plain"},
			expectedStatusCode: http.StatusPermanentRedirect,
			expectedLocation:   "https://example.com/other",
		},
		{
			name:                 "HTTP request with multiple bypass patterns should pass through for matching pattern",
			requestURL:           "http://example.com/metrics",
			tls:                  false,
			bypassPatterns:       []string{"/plain", "/api/health", "/metrics"},
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: "success",
		},
		{
			name:                 "HTTP request with bypass pattern should work with forwarded proto header set",
			requestURL:           "http://example.com/health",
			tls:                  false,
			forwardedProtoHeader: "X-Forwarded-Proto",
			forwardedProtoValue:  "http",
			bypassPatterns:       []string{"/health"},
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: "success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the ForceTLS handler
			forceTLS := &ForceTLS{
				ForwardedProtoHeader: tt.forwardedProtoHeader,
			}

			// Register bypass patterns
			for _, pattern := range tt.bypassPatterns {
				forceTLS.AllowBypass(pattern)
			}

			// Create a test handler that will be wrapped
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			})

			// Wrap the test handler with ForceTLS
			handler := forceTLS.Handle(testHandler)

			// Create the request
			req := httptest.NewRequest("GET", tt.requestURL, nil)

			// Set up TLS if needed
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}

			// Set forwarded proto header if provided
			if tt.forwardedProtoHeader != "" {
				req.Header.Set(tt.forwardedProtoHeader, tt.forwardedProtoValue)
			}

			// Create response recorder
			w := httptest.NewRecorder()

			// Serve the request
			handler.ServeHTTP(w, req)

			// Check status code
			if w.Code != tt.expectedStatusCode {
				t.Errorf("expected status code %d, got %d", tt.expectedStatusCode, w.Code)
			}

			// Check location header for redirects
			if tt.expectedLocation != "" {
				location := w.Header().Get("Location")
				if location != tt.expectedLocation {
					t.Errorf("expected location %s, got %s", tt.expectedLocation, location)
				}
			}

			// Check response body for non-redirects
			if tt.expectedResponseBody != "" {
				body := w.Body.String()
				if body != tt.expectedResponseBody {
					t.Errorf("expected body %s, got %s", tt.expectedResponseBody, body)
				}
			}
		})
	}
}
