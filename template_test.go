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
	"lds.li/web/internal/ctxkeys"
	"lds.li/web/static"
)

func TestTemplateFuncs(t *testing.T) {
	sfs := os.DirFS("static/testdata")
	sh, err := static.NewFileHandler(sfs, "/static")
	if err != nil {
		t.Fatal(err)
	}

	ctx := ctxkeys.ContextWithStaticHandler(t.Context(), sh)

	tmpl, err := template.New("test").Funcs(TemplateFuncs(ctx, nil)).Parse(`{{define "test"}}
StaticPath: {{StaticPath "subdir/file2.txt"}}
ScriptNonceAttr: {{ScriptNonceAttr}}
{{end}}`)
	if err != nil {
		t.Fatal(err)
	}

	base, _ := url.Parse("https://example.com")
	svr, err := NewServer(&Config{
		BaseURL:     base,
		Static:      sfs,
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

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Dest", "document")

	rr := httptest.NewRecorder()
	svr.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want status %d, got %d", http.StatusOK, rr.Code)
	}

	re := regexp.MustCompile(`(?m)^ScriptNonceAttr:\s*(\S+)`)
	matches := re.FindStringSubmatch(rr.Body.String())
	if len(matches) != 2 {
		t.Fatal("could not find script nonce value in response")
	}
	scriptNonce := matches[1]

	want := fmt.Sprintf(`
StaticPath: /static/subdir/file2.687830f0.txt
ScriptNonceAttr: %s
`, scriptNonce)

	if diff := cmp.Diff(
		strings.Split(cleanString(want), "\n"),
		strings.Split(cleanString(rr.Body.String()), "\n"),
	); diff != "" {
		t.Error(diff)
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
