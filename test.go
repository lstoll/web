package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/lstoll/web/session"
)

func TestBrowserRequest[T Session](_ testing.TB, w *Server[T], r *http.Request, sess T) (context.Context, *BrowserRequest) {
	ctx, _ := session.TestContext(w.Session(), r.Context(), sess)
	return ctx, &BrowserRequest{
		r: r.WithContext(ctx),
		// TODO - do we ever need to expose the result? It's mainly used for
		// fallback on middlewares.
		rw: httptest.NewRecorder(),
	}
}

type testSessType struct{} // TODO kill

func (s *testSessType) HasFlash() bool          { return false }
func (s *testSessType) FlashIsError() bool      { return false }
func (s *testSessType) FlashMessage() string    { return "" }
func (s *testSessType) SaveFlash(m string)      {}
func (s *testSessType) SaveErrorFlash(m string) {}

// TestWebServer returns a web server instance configured with normal defaults
// for this application
func TestWebServer(t testing.TB, opt ...func(c *Config[*testSessType])) *Server[*testSessType] {
	sstore, err := session.NewKVStore(session.NewMemoryKV(), nil)
	if err != nil {
		t.Fatal(err)
	}
	smgr, err := session.NewManager[*testSessType](sstore, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config[*testSessType]{
		BaseURL:        must(url.Parse("https://example.com")),
		SessionManager: smgr,
		Static:         nil, // TODO
		Templates:      nil, // TODO
		TemplateFuncs:  SampleTemplateFunctions(smgr),
	}
	svr, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return svr
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
