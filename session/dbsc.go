package session

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// verifyDBSCResponse checks if the Sec-Session-Response JWT is valid.
func verifyDBSCResponse(jwtStr string, pubKeyBytes []byte, expectedNonce string) error {
	parts := strings.Split(jwtStr, ".")
	if len(parts) != 3 {
		err := errors.New("invalid JWT format")
		return err
	}

	// Parse Header
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		err = fmt.Errorf("decoding header: %w", err)
		return err
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		err = fmt.Errorf("parsing header: %w", err)
		return err
	}
	if header.Alg != "ES256" {
		err = fmt.Errorf("unsupported algorithm: %s", header.Alg)
		return err
	}

	// Parse Payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		err = fmt.Errorf("decoding payload: %w", err)
		return err
	}
	var payload struct {
		Nonce string `json:"nonce"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		err = fmt.Errorf("parsing payload: %w", err)
		return err
	}
	if payload.Nonce != expectedNonce {
		err = errors.New("nonce mismatch")
		return err
	}

	// Parse Signature
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		err = fmt.Errorf("decoding signature: %w", err)
		return err
	}
	if len(sigBytes) != 64 {
		err = errors.New("invalid ES256 signature length")
		return err
	}

	// Verify Signature
	pubKey, err := x509.ParsePKIXPublicKey(pubKeyBytes)
	if err != nil {
		err = fmt.Errorf("parsing public key: %w", err)
		return err
	}
	ecdsaPubKey, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		err = errors.New("public key is not ECDSA")
		return err
	}

	hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))

	r := new(big.Int).SetBytes(sigBytes[:32])
	s := new(big.Int).SetBytes(sigBytes[32:])

	if !ecdsa.Verify(ecdsaPubKey, hash[:], r, s) {
		err = errors.New("invalid signature")
		return err
	}

	return nil
}
