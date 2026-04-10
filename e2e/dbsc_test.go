package e2e

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"lds.li/web/session"
)

// In-memory KV for the e2e test.
type memoryKV struct {
	data map[string][]byte
}

func (m *memoryKV) Get(ctx context.Context, key string) ([]byte, bool, error) {
	val, ok := m.data[key]
	return val, ok, nil
}

func (m *memoryKV) Set(ctx context.Context, key string, expiresAt time.Time, value []byte) error {
	m.data[key] = value
	return nil
}

func (m *memoryKV) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func networkHeader(headers network.Headers, name string) string {
	if headers == nil {
		return ""
	}
	for k, v := range headers {
		if strings.EqualFold(k, name) {
			switch t := v.(type) {
			case string:
				return t
			default:
				return fmt.Sprint(t)
			}
		}
	}
	return ""
}

func TestDBSC_E2E(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Chrome DBSC is not available on macOS in current releases; run this test on Windows (or re-enable when Apple/Chrome ship support).")
	}

	kv := &memoryKV{data: make(map[string][]byte)}
	opts := &session.ManagerOpts{
		IdleTimeout:         1 * time.Hour,
		DBSCRefreshInterval: 5 * time.Minute,
	}

	mgr, err := session.NewKVManager(kv, opts)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		sess := session.MustFromContext(r.Context())
		sess.Set("user", "alice")
		sess.InitiateDBSCRegistration(w, "/register")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("logged in")); err != nil {
			t.Errorf("write login body: %v", err)
		}
	})

	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		sess := session.MustFromContext(r.Context())
		if sess.PendingDBSCRegistrationChallenge() == "" {
			http.Error(w, "missing registration challenge on session", http.StatusBadRequest)
			return
		}

		dbscIDBytes := make([]byte, 16)
		if _, err := rand.Read(dbscIDBytes); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		dbscID := hex.EncodeToString(dbscIDBytes)

		if err := sess.CompleteDBSCRegistrationWithProof(r, dbscID); err != nil {
			t.Logf("registration proof rejected: %v", err)
			http.Error(w, "invalid registration proof", http.StatusUnauthorized)
			return
		}

		origin := "https://" + r.Host
		instructions := map[string]any{
			"session_identifier": dbscID,
			"refresh_url":        "/dbsc/refresh",
			"scope": map[string]any{
				"origin":        origin,
				"include_site":  false,
				"scope_specification": []any{},
			},
			"credentials": []map[string]string{{
				"type":       "cookie",
				"name":       "__Host-session-id",
				"attributes": "Path=/; Secure; HttpOnly; SameSite=Lax",
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(instructions); err != nil {
			t.Errorf("encode session instructions: %v", err)
		}
	})

	mux.HandleFunc("/dbsc/refresh", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/protected", func(w http.ResponseWriter, r *http.Request) {
		sess := session.MustFromContext(r.Context())
		if sess.Get("user") != "alice" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("protected content")); err != nil {
			t.Errorf("write protected body: %v", err)
		}
	})

	ts := httptest.NewTLSServer(mgr.Wrap(mux))
	defer ts.Close()

	co := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("enable-features", "DeviceBoundSessions"),
		chromedp.Flag("headless", "new"),
		chromedp.Flag("ignore-certificate-errors", "1"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), co...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	var registered bool
	var respondedToChallenge bool

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			url := e.Request.URL
			if strings.Contains(url, "/register") && e.Request.Method == http.MethodPost {
				registered = true
				t.Logf("saw registration POST %s", url)
			}
			if v := networkHeader(e.Request.Headers, "Secure-Session-Response"); v != "" {
				respondedToChallenge = true
				t.Logf("saw Secure-Session-Response on request to %s", url)
			} else if v := networkHeader(e.Request.Headers, "Sec-Session-Response"); v != "" {
				respondedToChallenge = true
				t.Logf("saw Sec-Session-Response on request to %s", url)
			}
		}
	})

	var body string
	err = chromedp.Run(ctx,
		network.Enable(),
		chromedp.Navigate(ts.URL+"/login"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			time.Sleep(3 * time.Second)
			return nil
		}),
		chromedp.Navigate(ts.URL+"/protected"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Text(`body`, &body, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("chromedp flow failed: %v", err)
	}

	if !registered {
		t.Fatal("Chrome did not POST the DBSC registration endpoint. Needs a recent Chrome with DeviceBoundSessions, HTTPS (here: httptest TLS), and Secure-Session-Registration support.")
	}
	if !respondedToChallenge {
		t.Fatal("Chrome did not send Secure-Session-Response on a follow-up request (expected after DBSC expiration + navigation to /protected).")
	}
	if !strings.Contains(body, "protected content") {
		t.Fatalf("expected protected page body to contain success text; got %q", body)
	}
}

// TestLoginResponseIncludesSecureSessionRegistration checks that the session
// stack emits DBSC registration headers over HTTPS (no Chrome DBSC required).
func TestLoginResponseIncludesSecureSessionRegistration(t *testing.T) {
	kv := &memoryKV{data: make(map[string][]byte)}
	mgr, err := session.NewKVManager(kv, &session.ManagerOpts{
		IdleTimeout:         time.Hour,
		DBSCRefreshInterval: 5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		s := session.MustFromContext(r.Context())
		s.Set("u", 1)
		s.InitiateDBSCRegistration(w, "/register")
		w.Write([]byte("ok"))
	})
	ts := httptest.NewTLSServer(mgr.Wrap(mux))
	defer ts.Close()
	c := ts.Client()
	c.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	resp, err := c.Get(ts.URL + "/login")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	reg := resp.Header.Get("Secure-Session-Registration")
	if reg == "" {
		t.Fatal("expected Secure-Session-Registration response header")
	}
	if !strings.Contains(reg, "ES256") || !strings.Contains(reg, "/register") || !strings.Contains(reg, "challenge=") {
		t.Fatalf("unexpected Secure-Session-Registration value: %q", reg)
	}
}
