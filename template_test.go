package web

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/lstoll/web/session"
	"github.com/lstoll/web/static"
)

func TestTemplateFuncs(t *testing.T) {
	sfs := os.DirFS("static/testdata")
	sh, err := static.NewFileHandler(sfs, "/static")
	if err != nil {
		t.Fatal(err)
	}

	ctx, _ := session.TestContext(t.Context(), nil)
	ctx = contextWithStaticHandler(ctx, sh)
	r := httptest.NewRequest("GET", "/test", nil)
	r = r.WithContext(ctx)

	tmpl, err := template.New("test").Funcs(TemplateFuncs(r, nil)).Parse(`{{define "test"}}
HasFlash: {{HasFlash}}
FlashIsError: {{FlashIsError}}
FlashMessage: {{FlashMessage}}
StaticPath: {{StaticPath "subdir/file2.txt"}}
ScriptNonceAttr: {{ScriptNonceAttr}}
{{end}}`)
	if err != nil {
		t.Fatal(err)
	}

	base, _ := url.Parse("https://example.com")
	svr, err := NewServer(&Config{
		BaseURL: base,
		// Templates:      tmpl,
		Static:      sfs, // TODO
		ScriptNonce: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	svr.Handle("/test", BrowserHandlerFunc(func(ctx context.Context, rw ResponseWriter, br *Request) error {
		return rw.WriteResponse(br, &TemplateResponse{
			Templates: tmpl,
			Name:      "test",
		})
	}))

	tests := []struct {
		name    string
		session func(*session.Session) *session.Session
		want    string
	}{
		{
			name: "empty session",
			want: `
HasFlash: false
FlashIsError: false
FlashMessage:
StaticPath: /static/subdir/file2.687830f0.txt
ScriptNonceAttr: %s
`,
		},
		{
			name: "flash error",
			session: func(s *session.Session) *session.Session {
				s.SetFlashError("an error occurred")
				return s
			},
			want: `
		HasFlash: true
		FlashIsError: true
		FlashMessage: an error occurred
		StaticPath: /static/subdir/file2.687830f0.txt
		ScriptNonceAttr: %s
		`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			// Add Sec-Fetch headers for our new protection
			req.Header.Set("Sec-Fetch-Site", "same-origin")
			req.Header.Set("Sec-Fetch-Mode", "navigate")
			req.Header.Set("Sec-Fetch-Dest", "document")

			var s *session.Session
			if tt.session != nil {
				s = tt.session(&session.Session{})
			}

			ctx, _ := session.TestContext(req.Context(), s)
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			svr.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("want status %d, got %d", http.StatusOK, rr.Code)
			}

			t.Logf("body: %s", rr.Body.String())

			// Now we only need to extract the script nonce
			re := regexp.MustCompile(`(?m)^ScriptNonceAttr:\s*(\S+)`)
			matches := re.FindStringSubmatch(rr.Body.String())
			if len(matches) != 2 {
				t.Fatal("could not find script nonce value in response")
			}
			scriptNonce := matches[1]

			// Only one formatting placeholder for the script nonce
			want := fmt.Sprintf(tt.want, scriptNonce)

			if diff := cmp.Diff(
				strings.Split(cleanString(want), "\n"),
				strings.Split(cleanString(rr.Body.String()), "\n"),
			); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func cleanString(input string) string {
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	trimmed := strings.Join(lines, "\n")

	decoded := html.UnescapeString(trimmed)

	return decoded
}
