package session

import (
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"maps"
	"net/http"
	"sync"
)

type sessionContextKey struct{}

// dbscServeConfigKey is used by Manager.Wrap to pass registration path into
// InitiateDBSCRegistration.
type dbscServeConfigKey struct{}

type dbscServeConfig struct {
	RegistrationPath string
}

// Session represents a tracked web session.
type Session struct {
	sessdata   persistedSession
	sessdataMu sync.RWMutex
	// datab is the original loaded data bytes. Used for idle timeout, when a
	// save may happen without data modification
	datab  []byte
	delete bool
	save   bool
	reset  bool
}

// Get returns the value for the given key from the session.
// If the key doesn't exist, it returns nil.
func (s *Session) Get(key string) any {
	s.sessdataMu.RLock()
	defer s.sessdataMu.RUnlock()

	return s.sessdata.Data[key]
}

// GetAll returns a copy of the session data map.
func (s *Session) GetAll() map[string]any {
	s.sessdataMu.RLock()
	defer s.sessdataMu.RUnlock()

	return maps.Clone(s.sessdata.Data)
}

// Set sets a single key-value pair in the session and marks it to be saved.
func (s *Session) Set(key string, value any) {
	s.sessdataMu.Lock()
	defer s.sessdataMu.Unlock()

	s.delete = false
	s.save = true
	s.sessdata.Data[key] = value
}

// SetAll sets the entire session data map and marks it to be saved.
func (s *Session) SetAll(data map[string]any) {
	s.sessdataMu.Lock()
	defer s.sessdataMu.Unlock()

	s.delete = false
	s.save = true

	s.sessdata.Data = data
}

// Delete marks the session for deletion at the end of the request.
func (s *Session) Delete() {
	s.sessdataMu.Lock()
	defer s.sessdataMu.Unlock()

	s.datab = nil
	s.sessdata = persistedSession{
		Data: make(map[string]any),
	}
	s.delete = true
	s.save = false
	s.reset = false
}

// Reset rotates the session ID to avoid session fixation.
func (s *Session) Reset() {
	s.sessdataMu.Lock()
	defer s.sessdataMu.Unlock()

	s.datab = nil
	s.save = false
	s.delete = false
	s.reset = true
}

// HasFlash indicates if there is a flash message.
func (s *Session) HasFlash() bool {
	return s.sessdata.Flash != flashLevelNone
}

// FlashIsError indicates that the flash message is an error.
func (s *Session) FlashIsError() bool {
	return s.sessdata.Flash == flashLevelError
}

// FlashMessage returns the current flash message and clears it.
func (s *Session) FlashMessage() string {
	flash := s.sessdata.FlashMsg
	if flash == "" {
		return ""
	}

	// Clear the flash, it's been read
	s.sessdata.FlashMsg = ""
	s.save = true

	return flash
}

func (s *Session) SetFlashError(message string) {
	s.sessdata.FlashMsg = message
	s.sessdata.Flash = flashLevelError
	s.save = true
}

func (s *Session) SetFlashMessage(message string) {
	s.sessdata.FlashMsg = message
	s.sessdata.Flash = flashLevelInfo
	s.save = true
}

// IsDeviceBound returns true if the session is cryptographically bound to a device.
func (s *Session) IsDeviceBound() bool {
	s.sessdataMu.RLock()
	defer s.sessdataMu.RUnlock()
	return len(s.sessdata.DBSCPublicJWKS) > 0
}

// InitiateDBSCRegistration adds Secure-Session-Registration immediately.
// When DBSC is enabled, the manager normally attaches this header automatically
// on the first HTTP response that persists session data (after Set/SetAll), as
// long as the session is not yet device-bound and has at least one key in Data.
// Call this if you need a registration offer without that save (e.g. empty Data)
// or to replace the current pending challenge.
//
// Requires DBSCRefreshInterval and DBSCRegistrationPath on the session Manager.
func (s *Session) InitiateDBSCRegistration(w http.ResponseWriter, r *http.Request) {
	cfg, ok := r.Context().Value(dbscServeConfigKey{}).(dbscServeConfig)
	if !ok || cfg.RegistrationPath == "" {
		http.Error(w, "DBSC registration path not configured on session manager", http.StatusInternalServerError)
		return
	}

	challengeBytes := make([]byte, 32)
	if _, err := rand.Read(challengeBytes); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	challenge := base64.RawURLEncoding.EncodeToString(challengeBytes)

	s.sessdataMu.Lock()
	s.sessdata.DBSCRegistrationChallenge = challenge
	s.save = true
	s.sessdataMu.Unlock()

	w.Header().Add("Secure-Session-Registration", `(ES256);path="`+cfg.RegistrationPath+`";challenge="`+challenge+`"`)
	slog.DebugContext(r.Context(), "dbsc InitiateDBSCRegistration",
		"path", cfg.RegistrationPath, "challenge_len", len(challenge))
}
