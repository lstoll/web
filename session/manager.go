package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// FromContext returns the Session from the given context. It panics if no
// session exists in the context.
func FromContext(ctx context.Context) (*Session, bool) {
	sessCtx, ok := ctx.Value(sessionContextKey{}).(*Session)
	if !ok {
		return nil, false
	}
	return sessCtx, true
}

// MustFromContext returns the Session from the given context. It panics if no
// session exists in the context.
func MustFromContext(ctx context.Context) *Session {
	sess, ok := FromContext(ctx)
	if !ok {
		panic("no session in context")
	}
	return sess
}

// storageMode identifies the session storage mechanism
type storageMode int

const (
	// storageModeCookie stores encrypted session data directly in cookies
	storageModeCookie storageMode = iota
	// storageModeKV stores session data in a KV store and session ID in cookies
	storageModeKV
)

// Manager handles both session data and storage.
type Manager struct {
	// Storage settings
	storageMode storageMode

	// Cookie-mode settings
	aead                AEAD
	compressionDisabled bool

	// KV-mode settings
	kv KV

	// Common settings
	cookieSettings SessionCookieOpts
	codec          codec
	opts           ManagerOpts
}

var DefaultIdleTimeout = 24 * time.Hour

// SessionCookieOpts configures cookie behavior for sessions
type SessionCookieOpts struct {
	Name     string
	Path     string
	Insecure bool
	Persist  bool
}

// newCookie creates a cookie with the configured options
func (c *SessionCookieOpts) newCookie(exp time.Time) *http.Cookie {
	hc := &http.Cookie{
		Name:     c.Name,
		Path:     c.Path,
		Secure:   !c.Insecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if c.Persist {
		hc.MaxAge = int(time.Until(exp).Seconds())
	}
	return hc
}

// ManagerOpts configures the session manager
type ManagerOpts struct {
	MaxLifetime time.Duration
	IdleTimeout time.Duration
	// Onload is called when a session is retrieved from storage
	Onload func(map[string]any) map[string]any
	// Cookie settings
	CookieOpts *SessionCookieOpts
}

// NewCookieManager creates a new Manager that stores session data in cookies
func NewCookieManager(aead AEAD, opts *ManagerOpts) (*Manager, error) {
	m := &Manager{
		storageMode: storageModeCookie,
		aead:        aead,
		opts: ManagerOpts{
			IdleTimeout: DefaultIdleTimeout,
		},
		codec: &gobCodec{},
	}

	if opts != nil {
		m.opts = *opts
	}

	if m.opts.IdleTimeout == 0 && m.opts.MaxLifetime == 0 {
		return nil, errors.New("at least one of idle timeout or max lifetime must be specified")
	}

	// Set cookie options
	if m.opts.CookieOpts != nil {
		m.cookieSettings = *m.opts.CookieOpts
	} else {
		m.cookieSettings = SessionCookieOpts{
			Name: "__Host-session",
			Path: "/",
		}
	}

	return m, nil
}

// NewKVManager creates a new Manager that stores session data in a KV store
func NewKVManager(kv KV, opts *ManagerOpts) (*Manager, error) {
	m := &Manager{
		storageMode: storageModeKV,
		kv:          kv,
		opts: ManagerOpts{
			IdleTimeout: DefaultIdleTimeout,
		},
		codec: &gobCodec{},
	}

	if opts != nil {
		m.opts = *opts
	}

	if m.opts.IdleTimeout == 0 && m.opts.MaxLifetime == 0 {
		return nil, errors.New("at least one of idle timeout or max lifetime must be specified")
	}

	// Set cookie options
	if m.opts.CookieOpts != nil {
		m.cookieSettings = *m.opts.CookieOpts
	} else {
		m.cookieSettings = SessionCookieOpts{
			Name: "__Host-session-id",
			Path: "/",
		}
	}

	return m, nil
}

// Constants for cookie format in the Manager
const (
	managerCookieMagic           = "EU1"
	managerCompressedCookieMagic = "EC1"
	managerCompressThreshold     = 512
	managerMaxCookieSize         = 4096
)

var managerCookieValueEncoding = base64.RawURLEncoding

// Wrap creates middleware that handles session management for each request
func (m *Manager) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Context().Value(sessionContextKey{}).(*Session); ok {
			panic("session middleware wrapped more than once")
		}

		// Create new session context with initial metadata
		sctx := &Session{
			sessdata: persistedSession{
				Data:      make(map[string]any),
				CreatedAt: time.Now(),
			},
		}

		// Load session data if it exists
		data, err := m.loadSession(r)
		if err != nil {
			// Log the error but don't fail the request - just start a new session
			slog.WarnContext(r.Context(), "Failed to load session, starting a new one", "err", err)
		} else if data != nil {
			// Try to decode the data
			decodedData, err := m.codec.Decode(data)
			if err != nil {
				// Log the error but don't fail the request - just start a new session
				slog.WarnContext(r.Context(), "Failed to decode session data, starting a new session", "err", err)
			} else {
				sctx.sessdata = decodedData

				// track the original data for idle timeout handling
				if m.opts.IdleTimeout != 0 {
					sctx.datab = data
				}

				if m.opts.Onload != nil {
					sctx.sessdata.Data = m.opts.Onload(sctx.sessdata.Data)
				}
			}
		}

		r = r.WithContext(context.WithValue(r.Context(), sessionContextKey{}, sctx))

		hw := &hookRW{
			ResponseWriter: w,
			hook:           m.saveHook(r, sctx),
		}

		next.ServeHTTP(hw, r)

		// if the handler doesn't write anything, make sure we fire the hook
		// anyway.
		hw.hookOnce.Do(func() {
			hw.hook(hw.ResponseWriter)
		})
	})
}

// Storage methods

// loadSession retrieves session data from the appropriate storage
func (m *Manager) loadSession(r *http.Request) ([]byte, error) {
	cookie, err := r.Cookie(m.cookieSettings.Name)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			// No session exists
			return nil, nil
		}
		return nil, fmt.Errorf("getting cookie %s: %w", m.cookieSettings.Name, err)
	}

	switch m.storageMode {
	case storageModeCookie:
		return m.loadFromCookie(cookie.Value)
	case storageModeKV:
		return m.loadFromKV(r.Context(), cookie.Value)
	default:
		return nil, fmt.Errorf("unknown storage mode: %v", m.storageMode)
	}
}

func (m *Manager) saveHook(r *http.Request, sctx *Session) func(w http.ResponseWriter) bool {
	return func(w http.ResponseWriter) bool {
		// Update the metadata timestamp
		sctx.sessdata.UpdatedAt = time.Now()

		// If we need to delete the session
		if sctx.delete || sctx.reset {
			if err := m.deleteSession(w, r, sctx); err != nil {
				m.handleErr(w, r, err)
				return false
			}
		}

		// If we need to save the session
		if sctx.save || sctx.reset {
			if err := m.saveSession(w, r, sctx); err != nil {
				m.handleErr(w, r, err)
				return false
			}
		} else if m.opts.IdleTimeout != 0 && len(sctx.datab) != 0 {
			// Just touch the session to update its lifetime
			if err := m.touchSession(w, r, sctx); err != nil {
				m.handleErr(w, r, err)
				return false
			}
		}

		return true
	}
}

// saveSession saves the session data to the appropriate storage
func (m *Manager) saveSession(w http.ResponseWriter, r *http.Request, sctx *Session) error {
	// Encode session data
	data, err := m.codec.Encode(sctx.sessdata)
	if err != nil {
		return fmt.Errorf("encoding session data: %w", err)
	}

	// Calculate expiry
	expiresAt := m.calculateExpiry(sctx.sessdata)

	switch m.storageMode {
	case storageModeCookie:
		return m.saveToCookie(w, r, expiresAt, data)
	case storageModeKV:
		return m.saveToKV(w, r, sctx, expiresAt, data)
	default:
		return fmt.Errorf("unknown storage mode: %v", m.storageMode)
	}
}

// deleteSession deletes the session from the appropriate storage
func (m *Manager) deleteSession(w http.ResponseWriter, r *http.Request, sctx *Session) error {
	// Delete cookie regardless of storage mode
	dc := m.cookieSettings.newCookie(time.Time{})
	dc.MaxAge = -1
	managerRemoveCookieByName(w, dc.Name)
	http.SetCookie(w, dc)

	// For KV mode, also delete from KV store
	if m.storageMode == storageModeKV {
		sessionID := getManagerSessionIDFromContext(r, m)
		if sessionID == "" {
			// Try to get from cookie
			cookie, err := r.Cookie(m.cookieSettings.Name)
			if err == nil {
				sessionID = cookie.Value
			}
		}

		if sessionID != "" {
			storeKey := managerHashSessionID(sessionID)
			if err := m.kv.Delete(r.Context(), storeKey); err != nil {
				return fmt.Errorf("deleting from KV: %w", err)
			}
		}

		// Generate a new ID for potential future use
		setManagerSessionIDInContext(r, m, rand.Text())
	}

	return nil
}

// touchSession updates the session expiry without modifying content
func (m *Manager) touchSession(w http.ResponseWriter, r *http.Request, sctx *Session) error {
	// Calculate new expiry
	expiresAt := m.calculateExpiry(sctx.sessdata)

	switch m.storageMode {
	case storageModeCookie:
		return m.saveToCookie(w, r, expiresAt, sctx.datab)
	case storageModeKV:
		// Get session ID
		sessionID := getManagerSessionIDFromContext(r, m)
		if sessionID == "" {
			cookie, err := r.Cookie(m.cookieSettings.Name)
			if err != nil {
				return nil // No session to touch
			}
			sessionID = cookie.Value
			setManagerSessionIDInContext(r, m, sessionID)
		}

		// Update KV expiry
		storeKey := managerHashSessionID(sessionID)
		if err := m.kv.Set(r.Context(), storeKey, expiresAt, sctx.datab); err != nil {
			return fmt.Errorf("updating KV expiry: %w", err)
		}

		// Update cookie expiry
		cookie := m.cookieSettings.newCookie(expiresAt)
		cookie.Value = sessionID

		managerRemoveCookieByName(w, cookie.Name)
		http.SetCookie(w, cookie)

		return nil
	default:
		return fmt.Errorf("unknown storage mode: %v", m.storageMode)
	}
}

func (m *Manager) calculateExpiry(sessdata persistedSession) time.Time {
	var invalidTimes []time.Time

	if m.opts.MaxLifetime != 0 {
		maxInvalidAt := sessdata.CreatedAt.Add(m.opts.MaxLifetime)
		invalidTimes = append(invalidTimes, maxInvalidAt)
	}

	if m.opts.IdleTimeout != 0 {
		var idleInvalidAt time.Time
		if !sessdata.UpdatedAt.IsZero() {
			idleInvalidAt = sessdata.UpdatedAt.Add(m.opts.IdleTimeout)
		} else {
			idleInvalidAt = sessdata.CreatedAt.Add(m.opts.IdleTimeout)
		}
		invalidTimes = append(invalidTimes, idleInvalidAt)
	}

	if len(invalidTimes) == 0 {
		return time.Time{}
	}

	earliestInvalidAt := invalidTimes[0]
	for _, t := range invalidTimes[1:] {
		if t.Before(earliestInvalidAt) {
			earliestInvalidAt = t
		}
	}

	return earliestInvalidAt
}

// Helper functions for tracking KV-mode session ID in context
type managerSessionIDCtxKey struct{ manager *Manager }

func getManagerSessionIDFromContext(r *http.Request, m *Manager) string {
	val := r.Context().Value(managerSessionIDCtxKey{manager: m})
	if val == nil {
		return ""
	}
	return val.(string)
}

func setManagerSessionIDInContext(r *http.Request, m *Manager, id string) {
	*r = *r.WithContext(context.WithValue(r.Context(), managerSessionIDCtxKey{manager: m}, id))
}

// Cookie handling helper
func managerRemoveCookieByName(w http.ResponseWriter, cookieName string) {
	headers := w.Header()
	setCookieHeaders := w.Header()["Set-Cookie"]

	if len(setCookieHeaders) == 0 {
		return
	}

	updatedCookies := []string{}
	for _, cookie := range setCookieHeaders {
		parts := strings.SplitN(cookie, "=", 2)
		if len(parts) > 0 && parts[0] != cookieName {
			updatedCookies = append(updatedCookies, cookie)
		}
	}

	headers.Del("Set-Cookie")
	for _, cookie := range updatedCookies {
		headers.Add("Set-Cookie", cookie)
	}
}

func (m *Manager) handleErr(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "error in session manager", "err", err)
	http.Error(w, "Internal Error", http.StatusInternalServerError)
}
