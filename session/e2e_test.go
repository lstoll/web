package session

import (
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"

	"golang.org/x/crypto/chacha20poly1305"
)

func TestE2E(t *testing.T) {
	aead, err := NewXChaPolyAEAD(genXChaPolyKey(), nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("KV Manager", func(t *testing.T) {
		mgr, err := NewKVManager(&memoryKV{contents: make(map[string]kvItem)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		runE2ETest(t, mgr, true)
	})

	t.Run("Cookie Manager", func(t *testing.T) {
		mgr, err := NewCookieManager(aead, nil)
		if err != nil {
			t.Fatal(err)
		}
		runE2ETest(t, mgr, false)
	})
}

func runE2ETest(t testing.TB, mgr *Manager, testReset bool) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /set", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, "query with no key", http.StatusInternalServerError)
			return
		}

		value := r.URL.Query().Get("value")
		if value == "" {
			t.Logf("query with no value")
			http.Error(w, "query with no value", http.StatusInternalServerError)
			return
		}

		// Log the key/value being set for debugging
		t.Logf("Setting session key=%s, value=%s", key, value)
		sess := MustFromContext(r.Context())
		sess.Set(key, value)
	})

	mux.HandleFunc("GET /get", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" {
			t.Fatal("query with no key")
		}

		// Log raw session data from context for debugging
		sessCtx := r.Context().Value(sessionContextKey{}).(*sessCtx)
		t.Logf("Session data in context: %+v", sessCtx.sessdata.Data)

		sess := MustFromContext(r.Context())
		value, ok := sess.Get(key).(string)
		if !ok {
			t.Logf("Key %s not found in session or not a string: %v", key, sess.Get(key))
			http.Error(w, "key not in session", http.StatusNotFound)
			return
		}

		_, _ = w.Write([]byte(value))
	})

	if testReset {
		mux.HandleFunc("GET /reset", func(w http.ResponseWriter, r *http.Request) {
			sess := MustFromContext(r.Context())
			sess.Reset()
		})
	}

	mux.HandleFunc("GET /clear", func(w http.ResponseWriter, r *http.Request) {
		sess := MustFromContext(r.Context())
		sess.Delete()
	})

	svr := httptest.NewTLSServer(mgr.Wrap(mux))
	t.Cleanup(svr.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	client := &http.Client{
		Transport: svr.Client().Transport,
		Jar:       jar,
	}

	for i := range 5 {
		doReq(t, client, svr.URL+fmt.Sprintf("/set?key=test%d&value=value%d", i, i), http.StatusOK)
	}

	// now ensure all 5 values are there
	for i := range 5 {
		resp := doReq(t, client, svr.URL+fmt.Sprintf("/get?key=test%d", i), http.StatusOK)
		if resp != fmt.Sprintf("value%d", i) {
			t.Fatalf("wanted returned value value%d, got: %s", i, resp)
		} else {
			t.Logf("got value%d", i)
		}
	}

	if testReset {
		// duplicate the jar, so after a reset we can make sure the old
		// session still can't be loaded.
		oldJar := must(cookiejar.New(nil))
		svrURL := must(url.Parse(svr.URL))
		oldJar.SetCookies(svrURL, jar.Cookies(svrURL))
		oldClient := &http.Client{
			Transport: svr.Client().Transport,
			Jar:       oldJar,
		}

		doReq(t, client, svr.URL+"/reset", http.StatusOK)
		doReq(t, client, svr.URL+"/get?key=test1", http.StatusOK)

		// this should fail, as the old session should no longer be accessible under
		// this ID.
		doReq(t, oldClient, svr.URL+"/get?key=test1", http.StatusNotFound)

		// clear it, and make sure it doesn't work
		for _, c := range []*http.Client{client, oldClient} {
			doReq(t, c, svr.URL+"/clear", http.StatusOK)
			doReq(t, c, svr.URL+"/get?key=test1", http.StatusNotFound)
			doReq(t, c, svr.URL+"/get?key=reset1", http.StatusNotFound)
		}
	}
}

func doReq(t testing.TB, client *http.Client, url string, wantStatus int) string {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	// Log cookies being sent
	t.Logf("Request cookies for %s: %v", url, req.Cookies())

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("error in request to %s: %v", url, err)
	}
	bb, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}

	// Log response cookies
	t.Logf("Response cookies from %s: %v", url, resp.Cookies())

	if resp.StatusCode != wantStatus {
		t.Logf("body: %s", string(bb))
		t.Fatalf("non-%d response status: %d", wantStatus, resp.StatusCode)
	}
	assertNoDuplicateCookies(t, resp.Cookies())
	return string(bb)
}

func assertNoDuplicateCookies(t testing.TB, cookies []*http.Cookie) {
	t.Helper()

	seen := make(map[string]struct{})
	for _, cookie := range cookies {
		if _, exists := seen[cookie.Name]; exists {
			t.Errorf("cookie %s has multiple set's", cookie.Name)
		}
		seen[cookie.Name] = struct{}{}
	}
}

func genXChaPolyKey() []byte {
	k := make([]byte, chacha20poly1305.KeySize)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		panic(err)
	}
	return k
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(fmt.Sprintf("error: %v", err))
	}
	return v
}
