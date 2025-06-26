package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

type KV interface {
	Get(_ context.Context, key string) (_ []byte, found bool, _ error)
	Set(_ context.Context, key string, expiresAt time.Time, value []byte) error
	Delete(_ context.Context, key string) error
}

// saveToKV saves session data to the KV store and puts the ID in a cookie
func (m *Manager) saveToKV(w http.ResponseWriter, r *http.Request, sctx *Session, expiresAt time.Time, data []byte) error {
	// Generate or get session ID
	sessionID := getManagerSessionIDFromContext(r, m)
	if sessionID == "" || sctx.reset {
		sessionID = rand.Text()
		setManagerSessionIDInContext(r, m, sessionID)
	}

	// Hash the session ID for storage
	storeKey := managerHashSessionID(sessionID)

	// Store in KV
	if err := m.kv.Set(r.Context(), storeKey, expiresAt, data); err != nil {
		return fmt.Errorf("storing in KV: %w", err)
	}

	// Set session ID cookie
	cookie := m.cookieSettings.newCookie(expiresAt)
	cookie.Value = sessionID

	managerRemoveCookieByName(w, cookie.Name)
	http.SetCookie(w, cookie)

	return nil
}

// loadFromKV loads session data from the KV store using the ID from the cookie
func (m *Manager) loadFromKV(ctx context.Context, sessionID string) ([]byte, error) {
	// Hash the session ID for storage
	storeKey := managerHashSessionID(sessionID)

	// Get data from KV
	data, found, err := m.kv.Get(ctx, storeKey)
	if err != nil {
		return nil, fmt.Errorf("getting from KV: %w", err)
	}

	if !found {
		return nil, nil
	}

	return data, nil
}

// Generate a consistent hash of session ID for KV storage
func managerHashSessionID(id string) string {
	h := sha256.New()
	h.Write([]byte(id))
	return hex.EncodeToString(h.Sum(nil))
}
