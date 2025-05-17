package web

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/lstoll/web/session"
)

func TestServer(t *testing.T) {
	base, _ := url.Parse("https://example.com")

	sm, err := session.NewKVManager(session.NewMemoryKV(), nil)
	if err != nil {
		t.Fatal(err)
	}

	tmpl, err := template.New("test").Parse(`<!DOCTYPE html>Hello, {{.}}!`)
	if err != nil {
		t.Fatal(err)
	}

	svr, err := NewServer(&Config{
		BaseURL:        base,
		SessionManager: sm,
		Templates:      tmpl,
		Static:         testfs,
	})
	if err != nil {
		t.Fatal(err)
	}

	svr.Handle("/test", BrowserHandlerFunc(func(ctx context.Context, rw ResponseWriter, br *Request) error {
		return rw.WriteResponse(&TemplateResponse{
			Templates: tmpl,
			Name:      "test",
			Data:      "world",
		})
	}))

	svr.Handle("/json", BrowserHandlerFunc(func(ctx context.Context, rw ResponseWriter, br *Request) error {
		return rw.WriteResponse(&JSONResponse{
			Data: map[string]any{"hello": "world"},
		})
	}))

	svr.Handle("/err", BrowserHandlerFunc(func(ctx context.Context, rw ResponseWriter, br *Request) error {
		return errors.New("some error")
	}))

	svr.HandleRaw("/raw", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "raw")
	}))

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
		// wantHeaderValues checks both the existence of the header, and the value
		wantHeaderValues http.Header
	}{
		{
			name:       "template",
			path:       "/test",
			wantStatus: http.StatusOK,
			wantBody:   "<!DOCTYPE html>Hello, world!",
			wantHeaderValues: http.Header{
				"Content-Type":            []string{"text/html; charset=utf-8"},
				"Content-Security-Policy": []string{"default-src 'none'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self'; connect-src 'self'; font-src 'self'; base-uri 'self'; frame-ancestors 'none'; report-uri https://example.com/_/csp-reports"},
			},
		},
		{
			name:       "error",
			path:       "/err",
			wantStatus: http.StatusInternalServerError,
			wantBody:   "Internal Server Error\n",
		},
		{
			name:       "raw",
			path:       "/raw",
			wantStatus: http.StatusOK,
			wantBody:   "raw\n",
		},
		{
			name:       "static",
			path:       "/static/file1.txt",
			wantStatus: http.StatusOK,
			wantBody:   "This is static file one\n",
		},
		{
			name:       "json",
			path:       "/json",
			wantStatus: http.StatusOK,
			wantBody:   `{"hello":"world"}` + "\n",
			wantHeaderValues: http.Header{
				"Content-Type":            []string{"application/json"},
				"Content-Security-Policy": []string{"default-src 'none'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self'; connect-src 'self'; font-src 'self'; base-uri 'self'; frame-ancestors 'none'; report-uri https://example.com/_/csp-reports"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tt.path, nil)
			rr := httptest.NewRecorder()
			svr.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("want status %d, got %d", tt.wantStatus, rr.Code)
			}

			if rr.Body.String() != tt.wantBody {
				t.Errorf("want body %q, got %q", tt.wantBody, rr.Body.String())
			}

			if tt.wantHeaderValues != nil {
				for k, v := range tt.wantHeaderValues {
					if diff := cmp.Diff(v, rr.Header()[k]); diff != "" {
						t.Errorf("headers mismatch (-want +got):\n%s", diff)

					}
				}
			}
		})
	}
}
