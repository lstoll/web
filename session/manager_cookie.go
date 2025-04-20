package session

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// saveToCookie saves session data directly to a cookie
func (m *Manager) saveToCookie(w http.ResponseWriter, r *http.Request, expiresAt time.Time, data []byte) error {
	// Add expiry time to data
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(expiresAt.Unix()))
	dataWithExpiry := append(b, data...)

	// Apply compression if needed
	magic := managerCookieMagic
	if !m.compressionDisabled && len(dataWithExpiry) > managerCompressThreshold {
		cw := getCompressor()
		defer putCompressor(cw)

		b, err := cw.Compress(dataWithExpiry)
		if err != nil {
			return fmt.Errorf("compressing cookie: %w", err)
		}
		dataWithExpiry = b
		magic = managerCompressedCookieMagic
	}

	// Encrypt data with AEAD
	encryptedData, err := m.aead.Encrypt(dataWithExpiry, []byte(m.cookieSettings.Name))
	if err != nil {
		return fmt.Errorf("encrypting cookie failed: %w", err)
	}

	// Format cookie value
	cookieValue := magic + "." + managerCookieValueEncoding.EncodeToString(encryptedData)
	if len(cookieValue) > managerMaxCookieSize {
		return fmt.Errorf("cookie size %d is greater than max %d", len(cookieValue), managerMaxCookieSize)
	}

	// Set cookie
	cookie := m.cookieSettings.newCookie(expiresAt)
	cookie.Value = cookieValue

	managerRemoveCookieByName(w, cookie.Name)
	http.SetCookie(w, cookie)

	return nil
}

// loadFromCookie extracts and decrypts session data from a cookie value
func (m *Manager) loadFromCookie(cookieValue string) ([]byte, error) {
	// Split and validate format
	sp := strings.SplitN(cookieValue, ".", 2)
	if len(sp) != 2 {
		return nil, errors.New("cookie does not contain two . separated parts")
	}

	magic := sp[0]
	encodedData := sp[1]

	// Decode
	decodedData, err := managerCookieValueEncoding.DecodeString(encodedData)
	if err != nil {
		return nil, fmt.Errorf("decoding cookie string: %w", err)
	}

	// Validate magic
	if magic != managerCompressedCookieMagic && magic != managerCookieMagic {
		return nil, fmt.Errorf("cookie has bad magic prefix: %s", magic)
	}

	// Decompress if needed
	if magic == managerCompressedCookieMagic {
		cr := getDecompressor()
		defer putDecompressor(cr)
		b, err := cr.Decompress(decodedData)
		if err != nil {
			return nil, fmt.Errorf("decompressing cookie: %w", err)
		}
		decodedData = b
	}

	// Decrypt using cookie name as associated data
	decryptedData, err := m.aead.Decrypt(decodedData, []byte(m.cookieSettings.Name))
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
