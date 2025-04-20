package session

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
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

// aesGCMAEAD is a implementation of the AEAD interface cookies are secured
// with, that uses AES-GCM with a random nonce. A single key should not be used
// for more than 4 billion encryptions. It is reccomended that tink with an
// automated key rotation is used, this is provided for simple use cases.
type aesGCMAEAD struct {
	encryptionKey  []byte
	decryptionKeys [][]byte
}

// newAESGCMAEAD constructs an AESGCMAEAD. The keys must be either 16, 24 or 32
// bytes. The encryption key is used as the primary encrypt/decrypt key.
// Additional decryption-only keys can be provided, to enable key rotation.
func newAESGCMAEAD(encryptionKey []byte, additionalDecryptionKeys [][]byte) (AEAD, error) {
	for _, k := range append([][]byte{encryptionKey}, additionalDecryptionKeys...) {
		if len(k) != 16 && len(k) != 24 && len(k) != 32 {
			return nil, errors.New("keys must be 16, 24, or 32 bytes")
		}
	}

	return &aesGCMAEAD{
		encryptionKey:  encryptionKey,
		decryptionKeys: additionalDecryptionKeys,
	}, nil
}

func (a *aesGCMAEAD) Encrypt(plaintext, associatedData []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM cipher: %w", err)
	}

	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err)
	}
	return append(nonce, aesgcm.Seal(nil, nonce, plaintext, associatedData)...), nil
}

func (a *aesGCMAEAD) Decrypt(ciphertext, associatedData []byte) ([]byte, error) {
	if len(ciphertext) < 12 {
		return nil, errors.New("invalid ciphertext")
	}
	var plaintext []byte
	for _, dk := range append([][]byte{a.encryptionKey}, a.decryptionKeys...) {
		block, err := aes.NewCipher(dk)
		if err != nil {
			return nil, fmt.Errorf("creating AES cipher: %w", err)
		}

		aesgcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("creating GCM cipher: %w", err)
		}

		pt, err := aesgcm.Open(nil, ciphertext[:12], ciphertext[12:], associatedData)
		if err != nil {
			continue
		}

		plaintext = pt
	}
	if plaintext == nil {
		return nil, fmt.Errorf("failed to decrypt data")
	}
	return plaintext, nil
}
