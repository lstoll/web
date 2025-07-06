package proxyhdrs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRemoteIP_Handle(t *testing.T) {
	tests := []struct {
		name                    string
		forwardedIPHeader       string
		forwardedIPHeaderFormat ForwardedIPHeaderFormat
		requestIP               string
		headerValue             string
		expectedRemoteAddr      string
		shouldModifyRemoteAddr  bool
	}{
		{
			name:                    "X-Forwarded-For with exact format",
			forwardedIPHeader:       ForwardedIPHeaderXFF,
			forwardedIPHeaderFormat: ForwardedIPHeaderFormatExact,
			requestIP:               "192.168.1.1:8080",
			headerValue:             "203.0.113.1",
			expectedRemoteAddr:      "203.0.113.1",
			shouldModifyRemoteAddr:  true,
		},
		{
			name:                    "X-Real-IP with exact format",
			forwardedIPHeader:       ForwardedIPHeaderXRealIP,
			forwardedIPHeaderFormat: ForwardedIPHeaderFormatExact,
			requestIP:               "192.168.1.1:8080",
			headerValue:             "203.0.113.2",
			expectedRemoteAddr:      "203.0.113.2",
			shouldModifyRemoteAddr:  true,
		},
		{
			name:                    "Fly-Client-IP with exact format",
			forwardedIPHeader:       ForwardedIPHeaderFlyClientIP,
			forwardedIPHeaderFormat: ForwardedIPHeaderFormatExact,
			requestIP:               "192.168.1.1:8080",
			headerValue:             "203.0.113.3",
			expectedRemoteAddr:      "203.0.113.3",
			shouldModifyRemoteAddr:  true,
		},
		{
			name:                    "X-Forwarded-For with first format",
			forwardedIPHeader:       ForwardedIPHeaderXFF,
			forwardedIPHeaderFormat: ForwardedIPHeaderFormatFirst,
			requestIP:               "192.168.1.1:8080",
			headerValue:             "203.0.113.1, 10.0.0.1, 172.16.0.1",
			expectedRemoteAddr:      "203.0.113.1",
			shouldModifyRemoteAddr:  true,
		},
		{
			name:                    "X-Forwarded-For with last format",
			forwardedIPHeader:       ForwardedIPHeaderXFF,
			forwardedIPHeaderFormat: ForwardedIPHeaderFormatLast,
			requestIP:               "192.168.1.1:8080",
			headerValue:             "203.0.113.1, 10.0.0.1, 172.16.0.1",
			expectedRemoteAddr:      "172.16.0.1",
			shouldModifyRemoteAddr:  true,
		},
		{
			name:                    "Empty header should not modify remote addr",
			forwardedIPHeader:       ForwardedIPHeaderXFF,
			forwardedIPHeaderFormat: ForwardedIPHeaderFormatExact,
			requestIP:               "192.168.1.1:8080",
			headerValue:             "",
			expectedRemoteAddr:      "192.168.1.1:8080",
			shouldModifyRemoteAddr:  false,
		},
		{
			name:                    "Invalid IP should not modify remote addr",
			forwardedIPHeader:       ForwardedIPHeaderXFF,
			forwardedIPHeaderFormat: ForwardedIPHeaderFormatExact,
			requestIP:               "192.168.1.1:8080",
			headerValue:             "invalid-ip",
			expectedRemoteAddr:      "192.168.1.1:8080",
			shouldModifyRemoteAddr:  false,
		},
		{
			name:                    "Empty forwarded header should not modify remote addr",
			forwardedIPHeader:       "",
			forwardedIPHeaderFormat: ForwardedIPHeaderFormatExact,
			requestIP:               "192.168.1.1:8080",
			headerValue:             "203.0.113.1",
			expectedRemoteAddr:      "192.168.1.1:8080",
			shouldModifyRemoteAddr:  false,
		},
		{
			name:                    "IPv6 address with exact format",
			forwardedIPHeader:       ForwardedIPHeaderXFF,
			forwardedIPHeaderFormat: ForwardedIPHeaderFormatExact,
			requestIP:               "[::1]:8080",
			headerValue:             "2001:db8::1",
			expectedRemoteAddr:      "2001:db8::1",
			shouldModifyRemoteAddr:  true,
		},
		{
			name:                    "Comma-separated values with spaces using first format",
			forwardedIPHeader:       ForwardedIPHeaderXFF,
			forwardedIPHeaderFormat: ForwardedIPHeaderFormatFirst,
			requestIP:               "192.168.1.1:8080",
			headerValue:             "  203.0.113.1  ,  10.0.0.1  ,  172.16.0.1  ",
			expectedRemoteAddr:      "203.0.113.1",
			shouldModifyRemoteAddr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the RemoteIP handler
			remoteIP := &RemoteIP{
				ForwardedIPHeader:       tt.forwardedIPHeader,
				ForwardedIPHeaderFormat: tt.forwardedIPHeaderFormat,
			}

			// Create a test handler that captures the request
			var capturedRequest *http.Request
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			})

			// Wrap the test handler with RemoteIP
			handler := remoteIP.Handle(testHandler)

			// Create the request
			req := httptest.NewRequest("GET", "http://example.com/test", nil)
			req.RemoteAddr = tt.requestIP

			// Set the forwarded IP header if provided
			if tt.forwardedIPHeader != "" && tt.headerValue != "" {
				req.Header.Set(tt.forwardedIPHeader, tt.headerValue)
			}

			// Create response recorder
			w := httptest.NewRecorder()

			// Serve the request
			handler.ServeHTTP(w, req)

			// Check that the handler was called
			if capturedRequest == nil {
				t.Fatal("test handler was not called")
			}

			// Check remote addr
			if capturedRequest.RemoteAddr != tt.expectedRemoteAddr {
				t.Errorf("expected remote addr %s, got %s", tt.expectedRemoteAddr, capturedRequest.RemoteAddr)
			}

			// Check status code
			if w.Code != http.StatusOK {
				t.Errorf("expected status code %d, got %d", http.StatusOK, w.Code)
			}

			// Check response body
			body := w.Body.String()
			if body != "success" {
				t.Errorf("expected body 'success', got '%s'", body)
			}
		})
	}
}

func TestOriginalRequestFromContext(t *testing.T) {
	// Create the RemoteIP handler
	remoteIP := &RemoteIP{
		ForwardedIPHeader:       ForwardedIPHeaderXFF,
		ForwardedIPHeaderFormat: ForwardedIPHeaderFormatExact,
	}

	// Create a test handler that checks the context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the original request from context
		originalRequest, ok := OriginalRequestFromContext(r.Context())
		if !ok {
			t.Error("OriginalRequestFromContext returned false")
			return
		}

		// Check that the original request has the original remote addr
		if originalRequest.RemoteAddr != "192.168.1.1:8080" {
			t.Errorf("expected original remote addr 192.168.1.1:8080, got %s", originalRequest.RemoteAddr)
		}

		// Check that the current request has the modified remote addr
		if r.RemoteAddr != "203.0.113.1" {
			t.Errorf("expected modified remote addr 203.0.113.1, got %s", r.RemoteAddr)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Wrap the test handler with RemoteIP
	handler := remoteIP.Handle(testHandler)

	// Create the request
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.RemoteAddr = "192.168.1.1:8080"
	req.Header.Set(ForwardedIPHeaderXFF, "203.0.113.1")

	// Create response recorder
	w := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(w, req)

	// Check status code
	if w.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, w.Code)
	}
}

func TestOriginalRequestFromContext_NoContext(t *testing.T) {
	// Test that OriginalRequestFromContext returns false when no context is set
	ctx := context.Background()
	_, ok := OriginalRequestFromContext(ctx)
	if ok {
		t.Error("OriginalRequestFromContext should return false when no context is set")
	}
}
