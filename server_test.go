package web

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/lstoll/web/csp"
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
		CSPOpts: []csp.HandlerOpt{
			csp.DefaultSrc(`'none'`),
			csp.ScriptSrc(`'self' 'unsafe-inline'`),
			csp.StyleSrc(`'self' 'unsafe-inline'`),
			csp.ImgSrc(`'self'`),
			csp.ConnectSrc(`'self'`),
			csp.FontSrc(`'self'`),
			csp.BaseURI(`'self'`),
			csp.FrameAncestors(`'none'`)},
		BaseURL:        base,
		SessionManager: sm,
		Static:         os.DirFS("static/testdata"),
	})
	if err != nil {
		t.Fatal(err)
	}

	svr.Handle("/test", BrowserHandlerFunc(func(ctx context.Context, rw ResponseWriter, br *Request) error {
		return rw.WriteResponse(br, &TemplateResponse{
			Templates: tmpl,
			Name:      "test",
			Data:      "world",
		})
	}))

	svr.Handle("/json", BrowserHandlerFunc(func(ctx context.Context, rw ResponseWriter, br *Request) error {
		return rw.WriteResponse(br, &JSONResponse{
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

func TestCompareSpecificity(t *testing.T) {
	testCases := []struct {
		name string
		p1   string
		p2   string
		want int // +1 for p1, -1 for p2, 0 for equal
	}{
		// --- Equality ---
		{name: "Identical patterns", p1: "/users/{id}", p2: "/users/{id}", want: 0},

		// --- Rule 1: Host Specificity ---
		{name: "Host vs No Host", p1: "api.example.com/users", p2: "/users", want: 1},
		{name: "No Host vs Host", p1: "/users", p2: "api.example.com/users", want: -1},

		// --- Rule 2: Method Specificity ---
		{name: "Method vs No Method", p1: "GET /users", p2: "/users", want: 1},
		{name: "No Method vs Method", p1: "/users", p2: "POST /users", want: -1},

		// --- Rule 3: Path Segments ---
		{name: "More segments vs fewer", p1: "/users/profiles/settings", p2: "/users/profiles/", want: 1},
		{name: "Fewer segments vs more", p1: "/users/", p2: "/users/profiles", want: -1},
		{name: "Host rule wins over segments", p1: "api.com/a", p2: "/a/b/c/d", want: 1},
		{name: "Method rule wins over segments", p1: "GET /a", p2: "/a/b/c/d", want: 1},

		// --- Path Segment vs String Length ---
		{
			name: "Segment count is higher, but string length is shorter",
			p1:   "/a/b",  // 2 segments, len 4
			p2:   "/user", // 1 segment, len 5
			want: 1,       // p1 is more specific
		},
		{
			name: "Segment count is lower, but string length is longer",
			p1:   "/longname", // 1 segment, len 9
			p2:   "/a/b/c",    // 3 segments, len 6
			want: -1,          // p2 is more specific
		},
		{
			name: "Equal segments with different lengths",
			p1:   "/longname", // 1 segment
			p2:   "/short",    // 1 segment
			want: 0,           // Equal specificity by this rule
		},
		{
			name: "Trailing slash vs no trailing slash (equal segments)",
			p1:   "/a/b/",
			p2:   "/a/b",
			want: 0,
		},

		// --- Rule 4: Wildcard Specificity ---
		{name: "Exact vs Wildcard (equal segments)", p1: "/users/123", p2: "/users/{id}", want: 1},
		{name: "Wildcard vs Exact (equal segments)", p1: "/users/{id}", p2: "/users/123", want: -1},
		{name: "Fewer wildcards vs more (equal segments)", p1: "/users/{id}/profile", p2: "/users/{id}/{action}", want: 1},
		{name: "Special {$}", p1: "/users/{$}", p2: "/users/", want: 1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &http.Request{Method: "GET", Host: "example.com"}
			if tc.name == "Host rule wins over segments" {
				req.Host = "api.com"
			}
			got := compareSpecificity(tc.p1, tc.p2, req)
			if got != tc.want {
				t.Errorf("compareSpecificity(%q, %q) = %d; want %d", tc.p1, tc.p2, got, tc.want)
			}
		})
	}
}

func TestCompareSpecificityWithRequest(t *testing.T) {
	testCases := []struct {
		name    string
		p1      string
		p2      string
		req     *http.Request
		want    int // +1 for p1, -1 for p2, 0 for equal
		reverse bool
	}{
		{
			name: "Method match vs no method",
			p1:   "GET /path",
			p2:   "/path",
			req:  httptest.NewRequest("GET", "/path", nil),
			want: 1,
		},
		{
			name: "Host match vs no host",
			p1:   "example.com/path",
			p2:   "/path",
			req:  httptest.NewRequest("GET", "https://example.com/path", nil),
			want: 1,
		},
		{
			name: "Host match is better than method match",
			p1:   "example.com/path",
			p2:   "POST /path",
			req:  httptest.NewRequest("POST", "https://example.com/path", nil),
			want: 1,
		},
		{
			name: "Exact method match vs other method",
			p1:   "GET /path",
			p2:   "POST /path",
			req:  httptest.NewRequest("GET", "/path", nil),
			want: 1,
		},
		{
			name: "Path specificity still wins with host",
			p1:   "example.com/a/b",
			p2:   "example.com/a",
			req:  httptest.NewRequest("GET", "https://example.com/a/b", nil),
			want: 1,
		},
		{
			name: "Path specificity still wins with method",
			p1:   "GET /a/b",
			p2:   "GET /a",
			req:  httptest.NewRequest("GET", "/a/b", nil),
			want: 1,
		},
		{
			name: "Host presence vs no host (non-matching request host)",
			p1:   "api.com/a",
			p2:   "/a/b", // same segment count
			req:  httptest.NewRequest("GET", "https://other.com/a/b", nil),
			want: 1, // p1 has a host, so it's more specific
		},
		{
			name: "Catch-all wildcard vs named wildcard",
			p1:   "/users/{id}",
			p2:   "/users/{$}",
			req:  httptest.NewRequest("GET", "/users/123", nil),
			want: 1, // {id} is more specific than {$}
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := compareSpecificity(tc.p1, tc.p2, tc.req)
			if got != tc.want {
				t.Errorf("compareSpecificity(%q, %q) = %d; want %d", tc.p1, tc.p2, got, tc.want)
			}

			// Run the reverse case to ensure symmetry
			got = compareSpecificity(tc.p2, tc.p1, tc.req)
			if got != -tc.want {
				t.Errorf("compareSpecificity(%q, %q) = %d; want %d", tc.p2, tc.p1, got, -tc.want)
			}
		})
	}
}
