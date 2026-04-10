package session

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"math/big"
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

	sigBytes := make([]byte, 64)
	copy(sigBytes[32-len(rBytes):32], rBytes)
	copy(sigBytes[64-len(sBytes):64], sBytes)

	return base64.RawURLEncoding.EncodeToString(sigBytes)
}

func ecCoordBytes(n *big.Int, size int) []byte {
	b := n.Bytes()
	if len(b) > size {
		panic("coordinate exceeds field")
	}
	out := make([]byte, size)
	copy(out[size-len(b):], b)
	return out
}

// es256JWKJSON returns a JSON object (for embedding in a JWT header) for tests only.
func es256JWKJSON(t *testing.T, pub *ecdsa.PublicKey) string {
	t.Helper()
	if pub.Curve != elliptic.P256() {
		t.Fatal("P-256 only")
	}
	byteLen := (pub.Curve.Params().BitSize + 7) / 8
	x := ecCoordBytes(pub.X, byteLen)
	y := ecCoordBytes(pub.Y, byteLen)
	return `{"kty":"EC","crv":"P-256","alg":"ES256","x":"` +
		base64.RawURLEncoding.EncodeToString(x) + `","y":"` +
		base64.RawURLEncoding.EncodeToString(y) + `"}`
}

func challengeFromSecureSessionRegistration(t *testing.T, reg string) string {
	t.Helper()
	const prefix = `challenge="`
	i := strings.Index(reg, prefix)
	if i < 0 {
		t.Fatalf("no challenge in %q", reg)
	}
	i += len(prefix)
	j := strings.IndexByte(reg[i:], '"')
	if j < 0 {
		t.Fatalf("unterminated challenge in %q", reg)
	}
	return reg[i : i+j]
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

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}

	handler := mgr.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := FromContext(r.Context())
		if !ok {
			t.Fatal("no session in context")
		}

		switch r.URL.Path {
		case "/start":
			sess.InitiateDBSCRegistration(w, "/register")
			w.WriteHeader(http.StatusOK)
		case "/register":
			if err := sess.CompleteDBSCRegistrationWithProof(r, "test-session-id-123"); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}
	}))

	// 1. Start DBSC registration (challenge stored on session)
	req0 := httptest.NewRequest(http.MethodGet, "/start", nil)
	rr0 := httptest.NewRecorder()
	handler.ServeHTTP(rr0, req0)
	if rr0.Code != http.StatusOK {
		t.Fatalf("start: %v", rr0.Code)
	}
	reg := rr0.Header().Get("Secure-Session-Registration")
	regChallenge := challengeFromSecureSessionRegistration(t, reg)

	var sessionCookie *http.Cookie
	for _, c := range rr0.Result().Cookies() {
		if c.Name == "__Host-session-id" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session cookie after /start")
	}

	// 2. Registration POST with Tink-shaped proof (JWK in header)
	jwk := es256JWKJSON(t, &privKey.PublicKey)
	headerJSON := `{"alg":"ES256","typ":"dbsc+jwt","jwk":` + jwk + `}`
	payloadJSON := `{"jti":"` + regChallenge + `"}`
	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(headerJSON))
	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))
	sigB64 := generateJWTSignature(t, privKey, headerB64, payloadB64)
	regJWT := headerB64 + "." + payloadB64 + "." + sigB64

	req1 := httptest.NewRequest(http.MethodPost, "/register", nil)
	req1.AddCookie(sessionCookie)
	req1.Header.Set("Secure-Session-Response", regJWT)
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("registration failed: %v %s", rr1.Code, rr1.Body.String())
	}

	var sessionCookieAfterReg *http.Cookie
	for _, c := range rr1.Result().Cookies() {
		if c.Name == "__Host-session-id" {
			sessionCookieAfterReg = c
			break
		}
	}
	if sessionCookieAfterReg != nil && sessionCookieAfterReg.Value != "" {
		sessionCookie = sessionCookieAfterReg
	}

	// 3. Protected request — DBSC expiration forces challenge
	req2 := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req2.AddCookie(sessionCookie)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for expired DBSC session, got %v", rr2.Code)
	}
	challenge := rr2.Header().Get("Secure-Session-Challenge")
	if challenge == "" {
		t.Fatal("expected Secure-Session-Challenge header")
	}

	parts := strings.SplitN(challenge, ";", 2)
	challengeNonceRaw := strings.TrimSpace(parts[0])
	challengeNonce := challengeNonceRaw[1 : len(challengeNonceRaw)-1]

	req3 := httptest.NewRequest(http.MethodGet, "/protected", nil)

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

	// Refresh proof: no jwk in header (§9.10)
	refreshHeaderJSON := `{"alg":"ES256","typ":"dbsc+jwt"}`
	refreshPayloadJSON := `{"jti":"` + challengeNonce + `"}`
	rhB64 := base64.RawURLEncoding.EncodeToString([]byte(refreshHeaderJSON))
	rpB64 := base64.RawURLEncoding.EncodeToString([]byte(refreshPayloadJSON))
	rsigB64 := generateJWTSignature(t, privKey, rhB64, rpB64)
	refreshJWT := rhB64 + "." + rpB64 + "." + rsigB64
	req3.Header.Set("Secure-Session-Response", refreshJWT)

	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusOK {
		t.Fatalf("expected 200 OK after valid DBSC challenge, got %v, body: %s", rr3.Code, rr3.Body.String())
	}
	if rr3.Body.String() != "ok" {
		t.Fatalf("expected body 'ok', got %q", rr3.Body.String())
	}

	req4 := httptest.NewRequest(http.MethodGet, "/protected", nil)

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
