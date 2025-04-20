package session

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
)

// AEAD defines the interface used for securing cookies. It matches the
// [github.com/tink-crypto/tink-go/v2/tink.AEAD] interface, and it is
// reccomended that tink is used to implement this.
type AEAD interface {
	// Encrypt the plaintext
	Encrypt(plaintext, associatedData []byte) ([]byte, error)

	// Decrypt the cipertext
	Decrypt(ciphertext, associatedData []byte) ([]byte, error)
}

// xchaPolyAEAD is an implementation of the AEAD interface that uses
// XChaCha20-Poly1305 with a random nonce. This provides 256-bit security
// and is resistant to timing attacks.
type xchaPolyAEAD struct {
	encryptionKey  []byte
	decryptionKeys [][]byte
}

// NewXChaPolyAEAD constructs an XChaCha20-Poly1305 AEAD. The keys must be 32 bytes.
// The encryption key is used as the primary encrypt/decrypt key.
// Additional decryption-only keys can be provided, to enable key rotation.
func NewXChaPolyAEAD(encryptionKey []byte, additionalDecryptionKeys [][]byte) (AEAD, error) {
	for _, k := range append([][]byte{encryptionKey}, additionalDecryptionKeys...) {
		if len(k) != chacha20poly1305.KeySize {
			return nil, fmt.Errorf("keys must be %d bytes", chacha20poly1305.KeySize)
		}
	}

	return &xchaPolyAEAD{
		encryptionKey:  encryptionKey,
		decryptionKeys: additionalDecryptionKeys,
	}, nil
}

func (x *xchaPolyAEAD) Encrypt(plaintext, associatedData []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(x.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("creating XChaCha20-Poly1305 cipher: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err)
	}

	return append(nonce, aead.Seal(nil, nonce, plaintext, associatedData)...), nil
}

func (x *xchaPolyAEAD) Decrypt(ciphertext, associatedData []byte) ([]byte, error) {
	nonceSize := chacha20poly1305.NonceSizeX
	if len(ciphertext) < nonceSize {
		return nil, errors.New("invalid ciphertext")
	}

	var plaintext []byte
	for _, dk := range append([][]byte{x.encryptionKey}, x.decryptionKeys...) {
		aead, err := chacha20poly1305.NewX(dk)
		if err != nil {
			return nil, fmt.Errorf("creating XChaCha20-Poly1305 cipher: %w", err)
		}

		pt, err := aead.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], associatedData)
		if err != nil {
			continue
		}

		plaintext = pt
		break
	}

	if plaintext == nil {
		return nil, fmt.Errorf("failed to decrypt data")
	}
	return plaintext, nil
}
