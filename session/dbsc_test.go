package session

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func generateJWTSignature(t *testing.T, privKey *ecdsa.PrivateKey, header, payload string) string {
	hash := sha256.Sum256([]byte(header + "." + payload))
	r, s, err := ecdsa.Sign(rand.Reader, privKey, hash[:])
	if err != nil {
		t.Fatalf("signing failed: %v", err)
	}

	rBytes := r.Bytes()
	sBytes := s.Bytes()

	// Pad with zeros to 32 bytes
	sigBytes := make([]byte, 64)
	copy(sigBytes[32-len(rBytes):32], rBytes)
	copy(sigBytes[64-len(sBytes):64], sBytes)

	return base64.RawURLEncoding.EncodeToString(sigBytes)
}

func TestDBSC(t *testing.T) {
	kv := &memoryKV{contents: make(map[string]kvItem)}

	opts := &ManagerOpts{
		IdleTimeout:         1 * time.Hour,
		DBSCRefreshInterval: 5 * time.Minute,
	}

	mgr, err := NewKVManager(kv, opts)
	if err != nil {
		t.Fatalf("creating manager: %v", err)
	}

	// 1. Generate an ES256 key pair
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		t.Fatalf("marshaling public key: %v", err)
	}

	// 2. Wrap a handler
	handler := mgr.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := FromContext(r.Context())
		if !ok {
			t.Fatal("no session in context")
		}

		// If we're setting up the test data:
		if r.URL.Path == "/register" {
			sess.CompleteDBSCRegistration(pubKeyBytes, "ES256", "test-session-id-123")
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	// Request 1: Registration
	req1 := httptest.NewRequest("POST", "/register", nil)
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("registration failed: %v", rr1.Code)
	}

	// Get session cookie
	var sessionCookie *http.Cookie
	for _, c := range rr1.Result().Cookies() {
		if c.Name == "__Host-session-id" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session cookie set")
	}

	// Request 2: Normal request (fails because expiration is set to Now() during CompleteDBSCRegistration)
	req2 := httptest.NewRequest("GET", "/protected", nil)
	req2.AddCookie(sessionCookie)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for expired DBSC session, got %v", rr2.Code)
	}
	challenge := rr2.Header().Get("Sec-Session-Challenge")
	if challenge == "" {
		t.Fatal("expected Sec-Session-Challenge header")
	}

	// Parse challenge header, e.g., "nonce_string"; id="session_id"
	parts := strings.SplitN(challenge, ";", 2)
	challengeNonceRaw := strings.TrimSpace(parts[0])

	// Remove quotes around the challenge nonce
	challengeNonce := challengeNonceRaw[1 : len(challengeNonceRaw)-1]
	// Request 3: Retry with valid Sec-Session-Response
	req3 := httptest.NewRequest("GET", "/protected", nil)

	// Read the new session cookie if one was set during the challenge
	var sessionCookie2 *http.Cookie
	for _, c := range rr2.Result().Cookies() {
		if c.Name == "__Host-session-id" {
			sessionCookie2 = c
			break
		}
	}
	if sessionCookie2 != nil && sessionCookie2.Value != "" {
		req3.AddCookie(sessionCookie2)
	} else {
		req3.AddCookie(sessionCookie)
	}

	headerJSON := `{"alg":"ES256"}`
	payloadJSON := `{"nonce":"` + challengeNonce + `"}`

	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(headerJSON))
	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))

	sigB64 := generateJWTSignature(t, privKey, headerB64, payloadB64)

	jwtStr := headerB64 + "." + payloadB64 + "." + sigB64
	req3.Header.Set("Sec-Session-Response", jwtStr)

	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusOK {
		t.Fatalf("expected 200 OK after valid DBSC challenge, got %v, body: %s", rr3.Code, rr3.Body.String())
	}
	if rr3.Body.String() != "ok" {
		t.Fatalf("expected body 'ok', got %q", rr3.Body.String())
	}

	// Request 4: Request again immediately, should not challenge
	req4 := httptest.NewRequest("GET", "/protected", nil)

	// Read the new session cookie if one was set during the success
	var sessionCookie3 *http.Cookie
	for _, c := range rr3.Result().Cookies() {
		if c.Name == "__Host-session-id" {
			sessionCookie3 = c
			break
		}
	}
	if sessionCookie3 != nil && sessionCookie3.Value != "" {
		req4.AddCookie(sessionCookie3)
	} else if sessionCookie2 != nil && sessionCookie2.Value != "" {
		req4.AddCookie(sessionCookie2)
	} else {
		req4.AddCookie(sessionCookie)
	}
	rr4 := httptest.NewRecorder()
	handler.ServeHTTP(rr4, req4)

	if rr4.Code != http.StatusOK {
		t.Fatalf("expected 200 OK without challenge, got %v", rr4.Code)
	}

	if sessionCookie2 == nil || sessionCookie2.Value == "" {
		t.Fatal("expected a new session cookie to be set after successful challenge")
	}
	if sessionCookie2.Value == sessionCookie.Value {
		t.Fatal("expected session cookie value to rotate after successful challenge")
	}
}
