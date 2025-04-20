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
)

func TestE2E(t *testing.T) {
	aead, err := newAESGCMAEAD(genAESKey(), nil)
	if err != nil {
		t.Fatal(err)
	}

	cookieStore := &cookieStore{
		AEAD:       aead,
		cookieOpts: defaultCookieStoreCookieOpts,
	}

	kvStore, err := NewKVStore(&memoryKV{contents: make(map[string]kvItem)}, nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("KV Manager, JSON", func(t *testing.T) {
		mgr, err := NewManager[jsonTestSession](kvStore, nil)
		if err != nil {
			t.Fatal(err)
		}
		runE2ETest(t, mgr, true)
	})

	t.Run("Cookie Manager, JSON", func(t *testing.T) {
		mgr, err := NewManager[jsonTestSession](cookieStore, nil)
		if err != nil {
			t.Fatal(err)
		}
		runE2ETest(t, mgr, false)
	})
}

type jsonTestSession struct {
	KV map[string]string `json:"map"`
}

func (j *jsonTestSession) GetMap() map[string]string {
	return j.KV
}

func (j *jsonTestSession) SetMap(m map[string]string) {
	j.KV = m
}

type codecAccessor interface {
	GetMap() map[string]string
	SetMap(map[string]string)
}

func runE2ETest[PtrT codecAccessor](t testing.TB, mgr *Manager[PtrT], testReset bool) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /set", func(w http.ResponseWriter, r *http.Request) {
		sess := mgr.Get(r.Context())

		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, "query with no key", http.StatusInternalServerError)
			return
		}

		value := r.URL.Query().Get("value")
		if key == "" {
			t.Logf("query with no value")
			http.Error(w, "query with no value", http.StatusInternalServerError)
			return
		}

		m := sess.GetMap()
		if m == nil {
			m = make(map[string]string)
		}

		m[key] = value

		sess.SetMap(m)

		mgr.Save(r.Context(), sess)
	})

	mux.HandleFunc("GET /get", func(w http.ResponseWriter, r *http.Request) {
		sess := mgr.Get(r.Context())

		key := r.URL.Query().Get("key")
		if key == "" {
			t.Fatal("query with no key")
		}

		value, ok := sess.GetMap()[key]
		if !ok {
			http.Error(w, "key not in session", http.StatusNotFound)
			return
		}

		_, _ = w.Write([]byte(value))
	})

	if testReset {
		mux.HandleFunc("GET /reset", func(w http.ResponseWriter, r *http.Request) {
			sess := mgr.Get(r.Context())
			mgr.Reset(r.Context(), sess)
		})
	}

	mux.HandleFunc("GET /clear", func(w http.ResponseWriter, r *http.Request) {
		mgr.Delete(r.Context())
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

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("error in request to %s: %v", url, err)
	}
	bb, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
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

func must[T any](v T, err error) T {
	if err != nil {
		panic(fmt.Sprintf("error: %v", err))
	}
	return v
}

func genAESKey() []byte {
	k := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		panic(err)
	}
	return k
}
