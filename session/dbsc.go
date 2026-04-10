package session

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

func trimDBSCStructuredString(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, `"`)
	s = strings.TrimSuffix(s, `"`)
	return s
}

type dbscJWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func parseECP256PublicKeyFromJWK(j *dbscJWK) (*ecdsa.PublicKey, error) {
	if j == nil || j.Kty != "EC" || j.Crv != "P-256" {
		return nil, errors.New("unsupported JWK (expected EC P-256)")
	}
	xBytes, err := base64.RawURLEncoding.DecodeString(j.X)
	if err != nil {
		return nil, fmt.Errorf("decoding JWK x: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(j.Y)
	if err != nil {
		return nil, fmt.Errorf("decoding JWK y: %w", err)
	}
	x := new(big.Int).SetBytes(xBytes)
	y := new(big.Int).SetBytes(yBytes)
	if !elliptic.P256().IsOnCurve(x, y) {
		return nil, errors.New("JWK point not on P-256")
	}
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
}

func jwkFromRegistrationJWTHeader(headerBytes []byte) (*dbscJWK, error) {
	var header struct {
		Alg string   `json:"alg"`
		Typ string   `json:"typ"`
		JWK *dbscJWK `json:"jwk"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, err
	}
	if header.Alg != "ES256" {
		return nil, fmt.Errorf("unsupported algorithm: %s", header.Alg)
	}
	if header.Typ != "" && header.Typ != "dbsc+jwt" {
		return nil, fmt.Errorf("unsupported typ: %s", header.Typ)
	}
	if header.JWK == nil {
		return nil, errors.New("missing jwk in JWT header")
	}
	return header.JWK, nil
}

// ParseDBSCRegistrationProof verifies a registration DBSC proof JWT (sent in
// Secure-Session-Response). It checks the signature with the embedded JWK,
// confirms jti matches expectedChallenge, and returns the public key in PKIX form.
func ParseDBSCRegistrationProof(jwtStr, expectedChallenge string) ([]byte, error) {
	jwtStr = trimDBSCStructuredString(jwtStr)
	parts := strings.Split(jwtStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid JWT format")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decoding header: %w", err)
	}
	jwk, err := jwkFromRegistrationJWTHeader(headerBytes)
	if err != nil {
		return nil, err
	}
	pubKey, err := parseECP256PublicKeyFromJWK(jwk)
	if err != nil {
		return nil, err
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decoding payload: %w", err)
	}
	var payload struct {
		JTI string `json:"jti"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("parsing payload: %w", err)
	}
	if payload.JTI != expectedChallenge {
		return nil, errors.New("jti does not match registration challenge")
	}

	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decoding signature: %w", err)
	}
	if len(sigBytes) != 64 {
		return nil, errors.New("invalid ES256 signature length")
	}

	hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r := new(big.Int).SetBytes(sigBytes[:32])
	s := new(big.Int).SetBytes(sigBytes[32:])
	if !ecdsa.Verify(pubKey, hash[:], r, s) {
		return nil, errors.New("invalid signature")
	}

	pubDER, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("marshaling public key: %w", err)
	}
	return pubDER, nil
}

// verifyDBSCResponse checks if the Secure-Session-Response JWT is valid.
func verifyDBSCResponse(jwtStr string, pubKeyBytes []byte, expectedChallenge string) error {
	jwtStr = trimDBSCStructuredString(jwtStr)
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
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		err = fmt.Errorf("parsing header: %w", err)
		return err
	}
	if header.Alg != "ES256" {
		err = fmt.Errorf("unsupported algorithm: %s", header.Alg)
		return err
	}
	if header.Typ != "" && header.Typ != "dbsc+jwt" {
		err = fmt.Errorf("unsupported typ: %s", header.Typ)
		return err
	}

	// Parse Payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		err = fmt.Errorf("decoding payload: %w", err)
		return err
	}
	var payload struct {
		JTI   string `json:"jti"`
		Nonce string `json:"nonce"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		err = fmt.Errorf("parsing payload: %w", err)
		return err
	}
	challenge := payload.JTI
	if challenge == "" {
		challenge = payload.Nonce
	}
	if challenge != expectedChallenge {
		err = errors.New("challenge mismatch (jti/nonce)")
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
