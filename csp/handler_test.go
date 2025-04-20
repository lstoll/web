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
