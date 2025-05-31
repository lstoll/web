package csp

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestHandler(t *testing.T) {
	baseURL := url.URL{
		Scheme: "http",
		Host:   "example.com",
	}

	cases := []struct {
		name          string
		opts          []HandlerOpt
		wrapped       http.Handler
		req           *http.Request
		checkResponse func(resp *http.Response) error
	}{
		{
			name: "pass through, report only",
			opts: []HandlerOpt{
				ReportOnly(true),
				DefaultSrc("http://example.com/default"),
				ScriptSrc("http://example.com/script"),
				StyleSrc("http://example.com/style"),
				ImgSrc("http://example.com/img"),
				FontSrc("http://example.com/font"),
				ObjectSrc("http://example.com/object"),
				MediaSrc("http://example.com/media"),
				BaseURI("http://example.com/base-uri"),
				FrameAncestors("http://example.com/frame-ancestors"),
				FormAction("http://example.com/form-action"),
			},
			wrapped: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("OK"))
			}),
			req: httptest.NewRequest(http.MethodGet, "http://example.com/foo", nil),
			checkResponse: func(resp *http.Response) error {
				want := `default-src http://example.com/default; script-src http://example.com/script; style-src http://example.com/style; img-src http://example.com/img; font-src http://example.com/font; object-src http://example.com/object; media-src http://example.com/media; base-uri http://example.com/base-uri; frame-ancestors http://example.com/frame-ancestors; form-action http://example.com/form-action; report-uri http://example.com/_/csp-reports`

				got := resp.Header.Get("Content-Security-Policy-Report-Only")
				if want != got {
					return fmt.Errorf("Content-Security-Policy-Report-Only: want: %v, got %v", want, got)
				}

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return err
				} else if !bytes.Equal([]byte("OK"), body) {
					return fmt.Errorf("body: want %v, got %v", []byte("OK"), body)
				}

				return nil
			},
		},
		{
			name: "pass through",
			opts: []HandlerOpt{
				DefaultSrc("http://example.com/default"),
				ScriptSrc("http://example.com/script"),
				StyleSrc("http://example.com/style"),
				ImgSrc("http://example.com/img"),
				FontSrc("http://example.com/font"),
				ObjectSrc("http://example.com/object"),
				MediaSrc("http://example.com/media"),
				BaseURI("http://example.com/base-uri"),
				FrameAncestors("http://example.com/frame-ancestors"),
				FormAction("http://example.com/form-action"),
			},
			wrapped: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("OK"))
			}),
			req: httptest.NewRequest(http.MethodGet, "http://example.com/foo", nil),
			checkResponse: func(resp *http.Response) error {
				want := `default-src http://example.com/default; script-src http://example.com/script; style-src http://example.com/style; img-src http://example.com/img; font-src http://example.com/font; object-src http://example.com/object; media-src http://example.com/media; base-uri http://example.com/base-uri; frame-ancestors http://example.com/frame-ancestors; form-action http://example.com/form-action; report-uri http://example.com/_/csp-reports`

				got := resp.Header.Get("Content-Security-Policy")
				if want != got {
					return fmt.Errorf("Content-Security-Policy: want: %v, got %v", want, got)
				}

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return err
				} else if !bytes.Equal([]byte("OK"), body) {
					return fmt.Errorf("body: want %v, got %v", []byte("OK"), body)
				}

				return nil
			},
		},
		{
			name: "report",
			opts: []HandlerOpt{
				DefaultSrc("http://example.com/default"),
				ScriptSrc("http://example.com/script"),
				StyleSrc("http://example.com/style"),
				ImgSrc("http://example.com/img"),
				FontSrc("http://example.com/font"),
				ObjectSrc("http://example.com/object"),
				MediaSrc("http://example.com/media"),
				BaseURI("http://example.com/base-uri"),
				FrameAncestors("http://example.com/frame-ancestors"),
				FormAction("http://example.com/form-action"),
			},
			wrapped: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("OK"))
			}),
			req: httptest.NewRequest(http.MethodPost, "http://example.com/_/csp-reports", bytes.NewReader([]byte("{}"))),
			checkResponse: func(resp *http.Response) error {
				want := `default-src http://example.com/default; script-src http://example.com/script; style-src http://example.com/style; img-src http://example.com/img; font-src http://example.com/font; object-src http://example.com/object; media-src http://example.com/media; base-uri http://example.com/base-uri; frame-ancestors http://example.com/frame-ancestors; form-action http://example.com/form-action; report-uri http://example.com/_/csp-reports`

				got := resp.Header.Get("Content-Security-Policy")
				if want != got {
					return fmt.Errorf("Content-Security-Policy: want: %v, got %v", want, got)
				}

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return err
				} else if len(body) > 0 {
					return fmt.Errorf("body: want empty, got %v", body)
				}

				return nil
			},
		},
		{
			name: "multiple sources",
			opts: []HandlerOpt{
				DefaultSrc("'self'"),
				ScriptSrc("https://scripts.example.com", "'unsafe-inline'"),
				ImgSrc("https://images.example.com", "data:"),
			},
			wrapped: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("OK"))
			}),
			req: httptest.NewRequest(http.MethodGet, "http://example.com/foo", nil),
			checkResponse: func(resp *http.Response) error {
				want := `default-src 'self'; script-src https://scripts.example.com 'unsafe-inline'; img-src https://images.example.com data:; report-uri http://example.com/_/csp-reports`
				got := resp.Header.Get("Content-Security-Policy")
				if want != got {
					return fmt.Errorf("Content-Security-Policy: want: %q, got %q", want, got)
				}
				return nil
			},
		},
		{
			name: "script and style nonces",
			opts: []HandlerOpt{
				ReportOnly(false),
				DefaultSrc("'self'"),
				ScriptSrc("'self'"), // Nonce will be added
				StyleSrc("'self'"),  // Nonce will be added
				WithScriptNonce(),
				WithStyleNonce(),
			},
			wrapped: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				scriptNonce, okS := GetScriptNonce(r.Context())
				if !okS {
					http.Error(w, "script nonce not found in context", http.StatusInternalServerError)
					return
				}
				styleNonce, okSt := GetStyleNonce(r.Context())
				if !okSt {
					http.Error(w, "style nonce not found in context", http.StatusInternalServerError)
					return
				}
				// Pass nonces to checkResponse via headers for simplicity in test
				w.Header().Set("X-Test-Script-Nonce", scriptNonce)
				w.Header().Set("X-Test-Style-Nonce", styleNonce)
				_, _ = w.Write([]byte("OK with nonces"))
			}),
			req: httptest.NewRequest(http.MethodGet, "http://example.com/nonce-test", nil),
			checkResponse: func(resp *http.Response) error {
				scriptNonceFromHandler := resp.Header.Get("X-Test-Script-Nonce")
				styleNonceFromHandler := resp.Header.Get("X-Test-Style-Nonce")

				if scriptNonceFromHandler == "" {
					return fmt.Errorf("did not get script nonce from handler via X-Test-Script-Nonce header")
				}
				if styleNonceFromHandler == "" {
					return fmt.Errorf("did not get style nonce from handler via X-Test-Style-Nonce header")
				}

				wantCSP := fmt.Sprintf("default-src 'self'; script-src 'self' 'nonce-%s'; style-src 'self' 'nonce-%s'; report-uri http://example.com/_/csp-reports", scriptNonceFromHandler, styleNonceFromHandler)
				gotCSP := resp.Header.Get("Content-Security-Policy")

				if wantCSP != gotCSP {
					return fmt.Errorf("Content-Security-Policy: want: %q, got %q", wantCSP, gotCSP)
				}

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return err
				}
				if !bytes.Equal([]byte("OK with nonces"), body) {
					return fmt.Errorf("body: want %q, got %q", "OK with nonces", body)
				}
				return nil
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cspHandler := NewHandler(baseURL, tc.opts...)
			handler := cspHandler.Wrap(tc.wrapped)

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, tc.req)

			resp := rr.Result()
			if err := tc.checkResponse(resp); err != nil {
				t.Fatal(err)
			}
		})
	}
}
