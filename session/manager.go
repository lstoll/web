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

// Session represents a user session with data access methods.
type Session interface {
	// Get returns the value for the given key from the session.
	// If the key doesn't exist, it returns nil.
	Get(key string) any

	// GetAll returns the entire session data map.
	GetAll() map[string]any

	// Set sets a single key-value pair in the session and marks it to be saved.
	Set(key string, value any)

	// SetAll sets the entire session data map and marks it to be saved.
	SetAll(data map[string]any)

	// Delete marks the session for deletion at the end of the request.
	Delete()

	// Reset rotates the session ID to avoid session fixation.
	Reset()

	// HasFlash indicates if there is a flash message.
	HasFlash() bool

	// FlashIsError indicates that the flash message is an error.
	FlashIsError() bool

	// FlashMessage returns the current flash message.
	FlashMessage() string
}

// sessionInstance implements the Session interface.
type sessionInstance struct {
	ctx context.Context
	mgr *Manager
}

func (s *sessionInstance) Get(key string) any {
	return s.mgr.get(s.ctx, key)
}

func (s *sessionInstance) GetAll() map[string]any {
	return s.mgr.getAll(s.ctx)
}

func (s *sessionInstance) Set(key string, value any) {
	s.mgr.set(s.ctx, key, value)
}

func (s *sessionInstance) SetAll(data map[string]any) {
	s.mgr.setAll(s.ctx, data)
}

func (s *sessionInstance) Delete() {
	s.mgr.delete(s.ctx)
}

func (s *sessionInstance) Reset() {
	s.mgr.reset(s.ctx)
}

func (s *sessionInstance) HasFlash() bool {
	return s.mgr.HasFlash(s.ctx)
}

func (s *sessionInstance) FlashIsError() bool {
	return s.mgr.FlashIsError(s.ctx)
}

func (s *sessionInstance) FlashMessage() string {
	return s.mgr.FlashMessage(s.ctx)
}

// sessionMetadata tracks additional information for the session manager to use,
// alongside the session data itself.
type sessionMetadata struct {
	CreatedAt time.Time
	UpdatedAt time.Time
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
		if _, ok := r.Context().Value(mgrSessCtxKey{inst: m}).(*sessCtx); ok {
			// already wrapped for this instance, noop
			next.ServeHTTP(w, r)
			return
		}

		// Create new session context with initial metadata
		md := &sessionMetadata{
			CreatedAt: time.Now(),
		}

		sctx := &sessCtx{
			metadata: md,
			data:     make(map[string]any),
		}

		// Store metadata in the map
		setMetadata(sctx.data, md)

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
				slog.WarnContext(r.Context(), "Failed to decode session data, starting a new one", "err", err)
			} else {
				sctx.data = decodedData
				sctx.metadata = extractMetadata(decodedData)

				// track the original data for idle timeout handling
				if m.opts.IdleTimeout != 0 {
					sctx.datab = data
				}

				if m.opts.Onload != nil {
					sctx.data = m.opts.Onload(sctx.data)
					// Update metadata in the map after potential modification
					setMetadata(sctx.data, sctx.metadata)
				}
			}
		}

		r = r.WithContext(context.WithValue(r.Context(), mgrSessCtxKey{inst: m}, sctx))

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

// GetSession returns a Session instance for the given context.
func (m *Manager) GetSession(ctx context.Context) Session {
	return &sessionInstance{
		ctx: ctx,
		mgr: m,
	}
}

// internal get implementation
func (m *Manager) get(ctx context.Context, key string) any {
	sessCtx, ok := ctx.Value(mgrSessCtxKey{inst: m}).(*sessCtx)
	if !ok {
		panic("context contained no or invalid session")
	}

	return sessCtx.data[key]
}

// internal getAll implementation
func (m *Manager) getAll(ctx context.Context) map[string]any {
	sessCtx, ok := ctx.Value(mgrSessCtxKey{inst: m}).(*sessCtx)
	if !ok {
		panic("context contained no or invalid session")
	}

	return sessCtx.data
}

// internal set implementation
func (m *Manager) set(ctx context.Context, key string, value any) {
	sessCtx, ok := ctx.Value(mgrSessCtxKey{inst: m}).(*sessCtx)
	if !ok {
		panic("context contained no or invalid session")
	}
	sessCtx.delete = false
	sessCtx.save = true
	sessCtx.data[key] = value
}

// internal setAll implementation
func (m *Manager) setAll(ctx context.Context, data map[string]any) {
	sessCtx, ok := ctx.Value(mgrSessCtxKey{inst: m}).(*sessCtx)
	if !ok {
		panic("context contained no or invalid session")
	}
	sessCtx.delete = false
	sessCtx.save = true

	// Keep the existing metadata
	md := sessCtx.metadata
	sessCtx.data = data

	// Make sure metadata stays in the map
	setMetadata(sessCtx.data, md)
}

// internal delete implementation
func (m *Manager) delete(ctx context.Context) {
	sessCtx, ok := ctx.Value(mgrSessCtxKey{inst: m}).(*sessCtx)
	if !ok {
		panic("context contained no or invalid session")
	}
	sessCtx.datab = nil
	sessCtx.data = make(map[string]any)
	sessCtx.delete = true
	sessCtx.save = false
	sessCtx.reset = false
}

// internal reset implementation
func (m *Manager) reset(ctx context.Context) {
	sessCtx, ok := ctx.Value(mgrSessCtxKey{inst: m}).(*sessCtx)
	if !ok {
		panic("context contained no or invalid session")
	}
	sessCtx.datab = nil
	sessCtx.save = false
	sessCtx.delete = false
	sessCtx.reset = true
}

func (m *Manager) handleErr(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "error in session manager", "err", err)
	http.Error(w, "Internal Error", http.StatusInternalServerError)
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

func (m *Manager) saveHook(r *http.Request, sctx *sessCtx) func(w http.ResponseWriter) bool {
	return func(w http.ResponseWriter) bool {
		// Update the metadata timestamp
		sctx.metadata.UpdatedAt = time.Now()
		setMetadata(sctx.data, sctx.metadata)

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
func (m *Manager) saveSession(w http.ResponseWriter, r *http.Request, sctx *sessCtx) error {
	// Encode session data
	data, err := m.codec.Encode(sctx.data)
	if err != nil {
		return fmt.Errorf("encoding session data: %w", err)
	}

	// Calculate expiry
	expiresAt := m.calculateExpiry(sctx.metadata)

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
func (m *Manager) deleteSession(w http.ResponseWriter, r *http.Request, sctx *sessCtx) error {
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
func (m *Manager) touchSession(w http.ResponseWriter, r *http.Request, sctx *sessCtx) error {
	// Calculate new expiry
	expiresAt := m.calculateExpiry(sctx.metadata)

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

func (m *Manager) calculateExpiry(md *sessionMetadata) time.Time {
	var invalidTimes []time.Time

	if m.opts.MaxLifetime != 0 {
		maxInvalidAt := md.CreatedAt.Add(m.opts.MaxLifetime)
		invalidTimes = append(invalidTimes, maxInvalidAt)
	}

	if m.opts.IdleTimeout != 0 {
		var idleInvalidAt time.Time
		if !md.UpdatedAt.IsZero() {
			idleInvalidAt = md.UpdatedAt.Add(m.opts.IdleTimeout)
		} else {
			idleInvalidAt = md.CreatedAt.Add(m.opts.IdleTimeout)
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

type mgrSessCtxKey struct{ inst *Manager }

type sessCtx struct {
	metadata *sessionMetadata
	// data is the actual session data
	data map[string]any
	// datab is the original loaded data bytes. Used for idle timeout, when a
	// save may happen without data modification
	datab  []byte
	delete bool
	save   bool
	reset  bool
}

// HasFlash indicates if there is a flash message
func (m *Manager) HasFlash(ctx context.Context) bool {
	sessCtx, ok := ctx.Value(mgrSessCtxKey{inst: m}).(*sessCtx)
	if !ok {
		return false
	}
	_, hasFlash := sessCtx.data["__flash"]
	return hasFlash
}

// FlashIsError indicates that the flash message is an error.
func (m *Manager) FlashIsError(ctx context.Context) bool {
	sessCtx, ok := ctx.Value(mgrSessCtxKey{inst: m}).(*sessCtx)
	if !ok {
		return false
	}
	isErr, ok := sessCtx.data["__flash_is_error"]
	if !ok {
		return false
	}
	boolVal, ok := isErr.(bool)
	if !ok {
		return false
	}
	return boolVal
}

// FlashMessage returns the current flash message and clears it.
func (m *Manager) FlashMessage(ctx context.Context) string {
	sessCtx, ok := ctx.Value(mgrSessCtxKey{inst: m}).(*sessCtx)
	if !ok {
		return ""
	}

	flash, ok := sessCtx.data["__flash"]
	if !ok {
		return ""
	}

	// Clear the flash
	delete(sessCtx.data, "__flash")
	delete(sessCtx.data, "__flash_is_error")
	sessCtx.save = true

	str, ok := flash.(string)
	if !ok {
		return ""
	}
	return str
}
