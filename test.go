package web

// func TestBrowserRequest(_ testing.TB, w *Server, r *http.Request, sess map[string]any) (context.Context, *Request) {
// 	ctx, _ := session.TestContext(w.Session(), r.Context(), sess)
// 	return ctx, &Request{
// 		r: r.WithContext(ctx),
// 	}
// }

// // TestWebServer returns a web server instance configured with normal defaults
// // for this application
// func TestWebServer(t testing.TB, opt ...func(c *Config)) *Server {
// 	smgr, err := session.NewKVManager(session.NewMemoryKV(), nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	cfg := &Config{
// 		BaseURL:        must(url.Parse("https://example.com")),
// 		SessionManager: smgr,
// 		Static:         nil, // TODO
// 		Templates:      nil, // TODO
// 		TemplateFuncs:  SampleTemplateFunctions(smgr),
// 	}
// 	svr, err := NewServer(cfg)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	return svr
// }

// func must[T any](v T, err error) T {
// 	if err != nil {
// 		panic(err)
// 	}
// 	return v
// }
