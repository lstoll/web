package web

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/lstoll/web/session"
)

func TestTemplateFuncs(t *testing.T) {
	sm, err := session.NewKVManager(session.NewMemoryKV(), nil)
	if err != nil {
		t.Fatal(err)
	}

	tmpl, err := template.New("test").Funcs(BaseFuncMap).Parse(`{{define "test"}}
HasFlash: {{HasFlash}}
FlashIsError: {{FlashIsError}}
FlashMessage: {{FlashMessage}}
CRSFField: {{CSRFField}}
CSRFToken: {{CSRFToken}}
StaticPath: {{StaticPath "subdir/file2.txt"}}
ScriptNonceAttr: {{ScriptNonceAttr}}
{{end}}`)
	if err != nil {
		t.Fatal(err)
	}

	base, _ := url.Parse("https://example.com")

	svr, err := NewServer(&Config{
		BaseURL:        base,
		SessionManager: sm,
		// Templates:      tmpl,
		Static:      testfs,
		ScriptNonce: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	svr.Handle("/test", BrowserHandlerFunc(func(ctx context.Context, rw ResponseWriter, br *Request) error {
		return rw.WriteResponse(&TemplateResponse{
			Templates: tmpl,
			Name:      "test",
		})
	}))

	tests := []struct {
		name    string
		session any
		want    string
	}{
		{
			name: "empty session",
			want: `
HasFlash: false
FlashIsError: false
FlashMessage:
CRSFField: <input id="csrf_token" type="hidden" name="csrf_token" value="">
CSRFToken:
StaticPath: /static/subdir/file2.txt?sum=687830f0aa1e6225
ScriptNonceAttr: %s
`,
		},
		/*{ // TODO
					name: "flash error",
					session: &testSession{
						flashIsError: true,
						flashMessage: "an error occurred",
					},
					want: `
		HasFlash: true
		FlashIsError: true
		FlashMessage: an error occurred
		CRSFField: <input id="csrf_token" type="hidden" name="csrf_token" value="">
		CSRFToken:
		StaticPath: /static/subdir/file2.txt?sum=687830f0aa1e6225
		ScriptNonceAttr: %s
		`,
				},*/
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.session == nil {
				tt.session = map[string]any{}
			}

			req := httptest.NewRequest("GET", "/test", nil)
			// Add Sec-Fetch headers for our new protection
			req.Header.Set("Sec-Fetch-Site", "same-origin")
			req.Header.Set("Sec-Fetch-Mode", "navigate")
			req.Header.Set("Sec-Fetch-Dest", "document")

			// ctx, _ := session.TestContext(sm, req.Context(), tt.session)
			// req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			svr.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("want status %d, got %d", http.StatusOK, rr.Code)
			}

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
