package session

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"

	"golang.org/x/crypto/chacha20poly1305"
)

func TestNewXChaPolyAEAD(t *testing.T) {
	tests := []struct {
		name          string
		encryptionKey []byte
		decryptKeys   [][]byte
		wantErr       bool
	}{
		{
			name:          "Valid key",
			encryptionKey: generateKey(t),
			wantErr:       false,
		},
		{
			name:          "Valid key with additional decryption keys",
			encryptionKey: generateKey(t),
			decryptKeys:   [][]byte{generateKey(t), generateKey(t)},
			wantErr:       false,
		},
		{
			name:          "Too short key",
			encryptionKey: make([]byte, chacha20poly1305.KeySize-1),
			wantErr:       true,
		},
		{
			name:          "Too long key",
			encryptionKey: make([]byte, chacha20poly1305.KeySize+1),
			wantErr:       true,
		},
		{
			name:          "Zero key",
			encryptionKey: make([]byte, chacha20poly1305.KeySize),
			wantErr:       true,
		},
		{
			name:          "Valid key with invalid decryption key",
			encryptionKey: generateKey(t),
			decryptKeys:   [][]byte{make([]byte, chacha20poly1305.KeySize-1)},
			wantErr:       true,
		},
		{
			name:          "Valid key with zero decryption key",
			encryptionKey: generateKey(t),
			decryptKeys:   [][]byte{make([]byte, chacha20poly1305.KeySize)},
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewXChaPolyAEAD(tt.encryptionKey, tt.decryptKeys)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewXChaPolyAEAD() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestXChaPolyAEAD_EncryptDecrypt(t *testing.T) {
	encryptionKey := generateKey(t)
	aead, err := NewXChaPolyAEAD(encryptionKey, nil)
	if err != nil {
		t.Fatalf("Failed to create AEAD: %v", err)
	}

	tests := []struct {
		name           string
		plaintext      []byte
		associatedData []byte
	}{
		{
			name:           "Simple plaintext, no associated data",
			plaintext:      []byte("hello world"),
			associatedData: []byte{},
		},
		{
			name:           "With associated data",
			plaintext:      []byte("hello world"),
			associatedData: []byte("session-id-12345"),
		},
		{
			name:           "Binary data",
			plaintext:      []byte{0x00, 0x01, 0x02, 0x03, 0x04},
			associatedData: []byte{0xFF, 0xFE, 0xFD},
		},
		{
			name:           "Large plaintext",
			plaintext:      bytes.Repeat([]byte("abcdefghijklmnopqrstuvwxyz"), 100),
			associatedData: []byte("large-text"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt the plaintext
			ciphertext, err := aead.Encrypt(tt.plaintext, tt.associatedData)
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			// Verify ciphertext is not same as plaintext
			if len(tt.plaintext) > 0 && bytes.Equal(ciphertext, tt.plaintext) {
				t.Errorf("Ciphertext should not equal plaintext")
			}

			// Verify ciphertext length (should be plaintext + nonce + tag)
			expectedLen := len(tt.plaintext) + chacha20poly1305.NonceSizeX + 16 // 16 is the poly1305 tag size
			if len(ciphertext) != expectedLen {
				t.Errorf("Ciphertext length = %d, expected %d", len(ciphertext), expectedLen)
			}

			// Decrypt the ciphertext
			decrypted, err := aead.Decrypt(ciphertext, tt.associatedData)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			// Verify decrypted equals original plaintext
			if !bytes.Equal(decrypted, tt.plaintext) {
				t.Errorf("Decrypt() = %v, want %v", decrypted, tt.plaintext)
			}
		})
	}
}

func TestXChaPolyAEAD_DecryptWithRotatedKeys(t *testing.T) {
	oldKey := generateKey(t)
	newKey := generateKey(t)

	// Create an AEAD with the old key as a decryption key
	aead, err := NewXChaPolyAEAD(newKey, [][]byte{oldKey})
	if err != nil {
		t.Fatalf("Failed to create AEAD: %v", err)
	}

	// Create an AEAD with just the old key to encrypt some data
	oldAead, err := NewXChaPolyAEAD(oldKey, nil)
	if err != nil {
		t.Fatalf("Failed to create old AEAD: %v", err)
	}

	plaintext := []byte("secret message")
	associatedData := []byte("session-context")

	// Encrypt with the old key
	ciphertext, err := oldAead.Encrypt(plaintext, associatedData)
	if err != nil {
		t.Fatalf("Encrypt() with old key error = %v", err)
	}

	// Decrypt with the new AEAD that has old key as decryption key
	decrypted, err := aead.Decrypt(ciphertext, associatedData)
	if err != nil {
		t.Fatalf("Decrypt() with new AEAD error = %v", err)
	}

	// Verify decrypted equals original plaintext
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypt() = %v, want %v", decrypted, plaintext)
	}

	// Now encrypt with the new key
	newCiphertext, err := aead.Encrypt(plaintext, associatedData)
	if err != nil {
		t.Fatalf("Encrypt() with new key error = %v", err)
	}

	// Verify old AEAD cannot decrypt new ciphertext
	_, err = oldAead.Decrypt(newCiphertext, associatedData)
	if err == nil {
		t.Error("Old AEAD should not be able to decrypt data encrypted with new key")
	}
}

func TestXChaPolyAEAD_DecryptInvalid(t *testing.T) {
	encryptionKey := generateKey(t)
	aead, err := NewXChaPolyAEAD(encryptionKey, nil)
	if err != nil {
		t.Fatalf("Failed to create AEAD: %v", err)
	}

	// Try to encrypt an empty plaintext to check behavior
	emptyPlaintext := []byte{}
	emptyAdData := []byte{}
	emptyCiphertext, err := aead.Encrypt(emptyPlaintext, emptyAdData)
	if err != nil {
		t.Logf("Note: Encrypting empty plaintext results in error: %v", err)
	}

	tests := []struct {
		name           string
		ciphertext     []byte
		associatedData []byte
		wantErr        bool
	}{
		{
			name:           "Empty ciphertext",
			ciphertext:     []byte{},
			associatedData: []byte{},
			wantErr:        true,
		},
		{
			name:           "Ciphertext too short",
			ciphertext:     make([]byte, chacha20poly1305.NonceSizeX-1),
			associatedData: []byte{},
			wantErr:        true,
		},
		{
			name:           "Invalid ciphertext (tampered)",
			ciphertext:     func() []byte { c, _ := aead.Encrypt([]byte("hello"), nil); c[len(c)-1]++; return c }(),
			associatedData: []byte{},
			wantErr:        true,
		},
		{
			name:           "Wrong associated data",
			ciphertext:     func() []byte { c, _ := aead.Encrypt([]byte("hello"), []byte("correct")); return c }(),
			associatedData: []byte("incorrect"),
			wantErr:        true,
		},
	}

	// Add empty plaintext test case if encryption succeeded
	if len(emptyCiphertext) > 0 {
		// Check if decryption works for empty plaintext
		emptyResult, emptyErr := aead.Decrypt(emptyCiphertext, emptyAdData)
		t.Logf("Empty plaintext decrypt result: %v, err: %v", emptyResult, emptyErr)

		tests = append(tests, struct {
			name           string
			ciphertext     []byte
			associatedData []byte
			wantErr        bool
		}{
			name:           "Empty plaintext ciphertext with wrong associated data",
			ciphertext:     emptyCiphertext,
			associatedData: []byte("wrong"),
			wantErr:        true,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := aead.Decrypt(tt.ciphertext, tt.associatedData)
			if (err != nil) != tt.wantErr {
				t.Errorf("Decrypt() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// generateKey generates a random key suitable for XChaCha20-Poly1305
func generateKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, chacha20poly1305.KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("Failed to generate random key: %v", err)
	}
	return key
}
