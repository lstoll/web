package session

import (
	"maps"
	"net/http"
	"sync"
	"time"
)

type sessionContextKey struct{}

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
	return len(s.sessdata.DBSCPublicKey) > 0
}

// InitiateDBSCRegistration instructs the browser to generate a new key pair and
// register it by making a request to the provided registration path.
func (s *Session) InitiateDBSCRegistration(w http.ResponseWriter, path string) {
	// The Sec-Session-Registration header specifies the path where the browser
	// should send the public key. We request ES256 as the algorithm.
	w.Header().Add("Sec-Session-Registration", `(ES256); path="`+path+`"`)
}

// CompleteDBSCRegistration binds the session to the provided public key.
func (s *Session) CompleteDBSCRegistration(publicKey []byte, alg string) {
	s.sessdataMu.Lock()
	defer s.sessdataMu.Unlock()

	s.sessdata.DBSCPublicKey = publicKey
	s.sessdata.DBSCAlgorithm = alg
	// Start with an expired token to force an immediate refresh challenge, or
	// set it to expire immediately. We will set it to time.Now() to effectively expire it.
	s.sessdata.DBSCExpiration = time.Now()

	s.save = true
}
