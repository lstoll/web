package session

type sessionContextKey struct{}

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

type sessCtx struct {
	sessdata persistedSession
	// datab is the original loaded data bytes. Used for idle timeout, when a
	// save may happen without data modification
	datab  []byte
	delete bool
	save   bool
	reset  bool
}

// Get returns the value for the given key from the session.
// If the key doesn't exist, it returns nil.
func (s *sessCtx) Get(key string) any {
	return s.sessdata.Data[key]
}

// GetAll returns the entire session data map.
func (s *sessCtx) GetAll() map[string]any {
	return s.sessdata.Data
}

// Set sets a single key-value pair in the session and marks it to be saved.
func (s *sessCtx) Set(key string, value any) {
	s.delete = false
	s.save = true
	s.sessdata.Data[key] = value
}

// SetAll sets the entire session data map and marks it to be saved.
func (s *sessCtx) SetAll(data map[string]any) {
	s.delete = false
	s.save = true

	s.sessdata.Data = data
}

// Delete marks the session for deletion at the end of the request.
func (s *sessCtx) Delete() {
	s.datab = nil
	s.sessdata = persistedSession{
		Data: make(map[string]any),
	}
	s.delete = true
	s.save = false
	s.reset = false
}

// Reset rotates the session ID to avoid session fixation.
func (s *sessCtx) Reset() {
	s.datab = nil
	s.save = false
	s.delete = false
	s.reset = true
}

// HasFlash indicates if there is a flash message.
func (s *sessCtx) HasFlash() bool {
	return s.sessdata.Flash != flashLevelNone
}

// FlashIsError indicates that the flash message is an error.
func (s *sessCtx) FlashIsError() bool {
	return s.sessdata.Flash == flashLevelError
}

// FlashMessage returns the current flash message and clears it.
func (s *sessCtx) FlashMessage() string {
	flash := s.sessdata.FlashMsg
	if flash == "" {
		return ""
	}

	// Clear the flash, it's been read
	s.sessdata.FlashMsg = ""
	s.save = true

	return flash
}
