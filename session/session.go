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

// Get returns the value for the given key from the session.
// If the key doesn't exist, it returns nil.
func (s *sessCtx) Get(key string) any {
	return s.data[key]
}

// GetAll returns the entire session data map.
func (s *sessCtx) GetAll() map[string]any {
	return s.data
}

// Set sets a single key-value pair in the session and marks it to be saved.
func (s *sessCtx) Set(key string, value any) {
	s.delete = false
	s.save = true
	s.data[key] = value
}

// SetAll sets the entire session data map and marks it to be saved.
func (s *sessCtx) SetAll(data map[string]any) {
	s.delete = false
	s.save = true

	// Keep the existing metadata
	md := s.metadata
	s.data = data

	// Make sure metadata stays in the map
	setMetadata(s.data, md)
}

// Delete marks the session for deletion at the end of the request.
func (s *sessCtx) Delete() {
	s.datab = nil
	s.data = make(map[string]any)
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
	_, hasFlash := s.data["__flash"]
	return hasFlash
}

// FlashIsError indicates that the flash message is an error.
func (s *sessCtx) FlashIsError() bool {
	isErr, ok := s.data["__flash_is_error"]
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
func (s *sessCtx) FlashMessage() string {
	flash, ok := s.data["__flash"]
	if !ok {
		return ""
	}

	// Clear the flash
	delete(s.data, "__flash")
	delete(s.data, "__flash_is_error")
	s.save = true

	str, ok := flash.(string)
	if !ok {
		return ""
	}
	return str
}
