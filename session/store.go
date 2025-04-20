package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var DefaultCookieTemplate = &http.Cookie{
	HttpOnly: true,
	Path:     "/",
	SameSite: http.SameSiteLaxMode,
}

// use https://github.com/golang/go/issues/67057#issuecomment-2261204789 when released
func newSID() string {
	b := make([]byte, 16)
	_, err := io.ReadFull(rand.Reader, b)
	if err != nil {
		panic(err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
}

// UnifiedCookieOpts can be used to customize the cookie used for tracking sessions.
type UnifiedCookieOpts struct {
	Name     string
	Path     string
	Insecure bool
	Persist  bool
}

func (c *UnifiedCookieOpts) newCookie(exp time.Time) *http.Cookie {
	hc := &http.Cookie{
		Name:     c.Name,
		Path:     c.Path,
		Secure:   !c.Insecure,
		HttpOnly: true,
	}
	if c.Persist {
		hc.MaxAge = int(time.Until(exp).Seconds())
	}
	return hc
}

// StorageBackend defines the actual storage mechanism for session data
type StorageBackend interface {
	// Store saves the session data and returns what should be stored in the cookie
	Store(ctx context.Context, id string, expiresAt time.Time, data []byte) (string, error)

	// Load retrieves session data from the backend using information from the cookie
	Load(ctx context.Context, cookieValue string) ([]byte, error)

	// Delete removes the session data
	Delete(ctx context.Context, cookieValue string) error
}

// UnifiedStore is the main implementation of session.Store that uses different
// storage backends
type UnifiedStore struct {
	backend    StorageBackend
	cookieOpts *UnifiedCookieOpts
}

// UnifiedStoreOpts configures options for the UnifiedStore
type UnifiedStoreOpts struct {
	CookieOpts *UnifiedCookieOpts
}

// NewUnifiedStore creates a new unified Store with the given backend
func NewUnifiedStore(backend StorageBackend, opts *UnifiedStoreOpts) *UnifiedStore {
	s := &UnifiedStore{
		backend: backend,
	}

	if opts != nil && opts.CookieOpts != nil {
		s.cookieOpts = opts.CookieOpts
	} else {
		// Default cookie settings
		s.cookieOpts = &UnifiedCookieOpts{
			Name: "__Host-session",
			Path: "/",
		}
	}

	return s
}

// GetSession implements Store.GetSession by retrieving data from the cookie and backend
func (s *UnifiedStore) GetSession(r *http.Request) ([]byte, error) {
	// Try to get the cookie
	cookie, err := r.Cookie(s.cookieOpts.Name)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			// No session exists
			return nil, nil
		}
		return nil, fmt.Errorf("getting cookie %s: %w", s.cookieOpts.Name, err)
	}

	// We have a cookie, load from the backend
	return s.backend.Load(r.Context(), cookie.Value)
}

// PutSession implements Store.PutSession by storing data in the backend and setting cookie
func (s *UnifiedStore) PutSession(w http.ResponseWriter, r *http.Request, expiresAt time.Time, data []byte) error {
	// Generate session ID if needed
	sessionID := getUnifiedSessionIDFromContext(r, s)
	if sessionID == "" {
		sessionID = newSID()
		setUnifiedSessionIDInContext(r, s, sessionID)
	}

	// Store in backend
	cookieValue, err := s.backend.Store(r.Context(), sessionID, expiresAt, data)
	if err != nil {
		return fmt.Errorf("storing session data: %w", err)
	}

	// Set cookie
	cookie := s.cookieOpts.newCookie(expiresAt)
	cookie.Value = cookieValue

	// Remove any existing cookie with the same name
	removeCookieByName(w, cookie.Name)
	http.SetCookie(w, cookie)

	return nil
}

// DeleteSession implements Store.DeleteSession by removing data from backend and cookie
func (s *UnifiedStore) DeleteSession(w http.ResponseWriter, r *http.Request) error {
	sessionID := getUnifiedSessionIDFromContext(r, s)
	if sessionID == "" {
		// Try to get from cookie
		cookie, err := r.Cookie(s.cookieOpts.Name)
		if err != nil {
			if errors.Is(err, http.ErrNoCookie) {
				// No session exists
				return nil
			}
			return fmt.Errorf("getting cookie %s: %w", s.cookieOpts.Name, err)
		}
		sessionID = cookie.Value
	}

	// Delete from backend
	if err := s.backend.Delete(r.Context(), sessionID); err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}

	// Delete cookie
	dc := s.cookieOpts.newCookie(time.Time{})
	dc.MaxAge = -1
	removeCookieByName(w, dc.Name)
	http.SetCookie(w, dc)

	// Generate a new ID for potential future use in this request
	setUnifiedSessionIDInContext(r, s, newSID())

	return nil
}

// Helper functions for tracking session ID in context
type unifiedSessionIDKey struct{ store *UnifiedStore }

func getUnifiedSessionIDFromContext(r *http.Request, s *UnifiedStore) string {
	val := r.Context().Value(unifiedSessionIDKey{store: s})
	if val == nil {
		return ""
	}
	return val.(string)
}

func setUnifiedSessionIDInContext(r *http.Request, s *UnifiedStore, id string) {
	*r = *r.WithContext(context.WithValue(r.Context(), unifiedSessionIDKey{store: s}, id))
}

// Cookie handling helper
func removeCookieByName(w http.ResponseWriter, cookieName string) {
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

// KVBackend implements StorageBackend using a key-value store
type KVBackend struct {
	kv KV
}

// NewKVBackend creates a new KV-based storage backend
func NewKVBackend(kv KV) *KVBackend {
	return &KVBackend{kv: kv}
}

func (k *KVBackend) Store(ctx context.Context, id string, expiresAt time.Time, data []byte) (string, error) {
	// Hash the ID for storage
	storeKey := hashSessionID(id)

	// Store in KV
	if err := k.kv.Set(ctx, storeKey, expiresAt, data); err != nil {
		return "", fmt.Errorf("storing in KV: %w", err)
	}

	// Return the plain session ID for the cookie
	return id, nil
}

func (k *KVBackend) Load(ctx context.Context, cookieValue string) ([]byte, error) {
	// Hash the ID from cookie
	storeKey := hashSessionID(cookieValue)

	// Get from KV
	data, found, err := k.kv.Get(ctx, storeKey)
	if err != nil {
		return nil, fmt.Errorf("getting from KV: %w", err)
	}

	if !found {
		return nil, nil
	}

	return data, nil
}

func (k *KVBackend) Delete(ctx context.Context, cookieValue string) error {
	storeKey := hashSessionID(cookieValue)
	return k.kv.Delete(ctx, storeKey)
}

// CookieBackend implements StorageBackend by storing encrypted data directly in cookies
type CookieBackend struct {
	aead                AEAD
	compressionDisabled bool
}

// Constants for cookie data
const (
	cookieMagic           = "EU1"
	compressedCookieMagic = "EC1"
	compressThreshold     = 512
	maxCookieSize         = 4096
)

var cookieValueEncoding = base64.RawURLEncoding

// NewCookieBackend creates a new cookie-based storage backend
func NewCookieBackend(aead AEAD, compressionDisabled bool) *CookieBackend {
	return &CookieBackend{
		aead:                aead,
		compressionDisabled: compressionDisabled,
	}
}

func (c *CookieBackend) Store(ctx context.Context, id string, expiresAt time.Time, data []byte) (string, error) {
	// Add expiry time to data
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(expiresAt.Unix()))
	dataWithExpiry := append(b, data...)

	// Apply compression if needed
	magic := cookieMagic
	if !c.compressionDisabled && len(dataWithExpiry) > compressThreshold {
		cw := getCompressor()
		defer putCompressor(cw)

		b, err := cw.Compress(dataWithExpiry)
		if err != nil {
			return "", fmt.Errorf("compressing cookie: %w", err)
		}
		dataWithExpiry = b
		magic = compressedCookieMagic
	}

	// Encrypt data with AEAD
	encryptedData, err := c.aead.Encrypt(dataWithExpiry, []byte(id))
	if err != nil {
		return "", fmt.Errorf("encrypting cookie failed: %w", err)
	}

	// Format cookie value
	cookieValue := magic + "." + cookieValueEncoding.EncodeToString(encryptedData)
	if len(cookieValue) > maxCookieSize {
		return "", fmt.Errorf("cookie size %d is greater than max %d", len(cookieValue), maxCookieSize)
	}

	return cookieValue, nil
}

func (c *CookieBackend) Load(ctx context.Context, cookieValue string) ([]byte, error) {
	// Split and validate format
	sp := strings.SplitN(cookieValue, ".", 2)
	if len(sp) != 2 {
		return nil, errors.New("cookie does not contain two . separated parts")
	}

	magic := sp[0]
	encodedData := sp[1]

	// Decode
	decodedData, err := cookieValueEncoding.DecodeString(encodedData)
	if err != nil {
		return nil, fmt.Errorf("decoding cookie string: %w", err)
	}

	// Validate magic
	if magic != compressedCookieMagic && magic != cookieMagic {
		return nil, fmt.Errorf("cookie has bad magic prefix: %s", magic)
	}

	// Decompress if needed
	if magic == compressedCookieMagic {
		cr := getDecompressor()
		defer putDecompressor(cr)
		b, err := cr.Decompress(decodedData)
		if err != nil {
			return nil, fmt.Errorf("decompressing cookie: %w", err)
		}
		decodedData = b
	}

	// Decrypt using cookie name as associated data
	decryptedData, err := c.aead.Decrypt(decodedData, []byte(""))
	if err != nil {
		return nil, fmt.Errorf("decrypting cookie: %w", err)
	}

	// Check expiry
	if len(decryptedData) < 8 {
		return nil, errors.New("decrypted data too short")
	}
	expiresAt := time.Unix(int64(binary.LittleEndian.Uint64(decryptedData[:8])), 0)
	if expiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("cookie expired at %s", expiresAt)
	}

	// Return actual data (without expiry)
	return decryptedData[8:], nil
}

func (c *CookieBackend) Delete(ctx context.Context, cookieValue string) error {
	// No backend storage to clean up
	return nil
}

// Helper function to generate a consistent hash of session ID for KV storage
func hashSessionID(id string) string {
	h := sha256.New()
	h.Write([]byte(id))
	return hex.EncodeToString(h.Sum(nil))
}
