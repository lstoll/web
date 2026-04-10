package session

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/tink-crypto/tink-go/v2/jwt"
	"github.com/tink-crypto/tink-go/v2/keyset"
)

const dbscClaimTyp = "dbsc+jwt"

func dbscValidator() (*jwt.Validator, error) {
	return jwt.NewValidator(&jwt.ValidatorOpts{
		IgnoreIssuer:           true,
		IgnoreAudiences:        true,
		ExpectedTypeHeader:     new(dbscClaimTyp),
		AllowMissingExpiration: true,
	})
}

// stripDBSCProofToken returns the JWT string from a Secure-Session-Response
// value (an sf-string, which may appear quoted in some intermediaries).
func stripDBSCProofToken(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// parseDBSCRegistrationProof verifies a registration DBSC proof using the JWK
// embedded in the JWT header (§9.10) and returns the verified claims plus the
// public keyset handle Tink used for verification.
func parseDBSCRegistrationProof(compact string) (*jwt.VerifiedJWT, *keyset.Handle, error) {
	hdrB64, _, _, ok := parseToken(compact)
	if !ok {
		return nil, nil, errors.New("invalid token format")
	}
	hdrJSON, err := base64.RawURLEncoding.DecodeString(hdrB64)
	if err != nil {
		return nil, nil, fmt.Errorf("decoding jwt header: %w", err)
	}

	var hdr map[string]json.RawMessage
	if err := json.Unmarshal(hdrJSON, &hdr); err != nil {
		return nil, nil, fmt.Errorf("unmarshalling header: %w", err)
	}
	jwkRaw, ok := hdr["jwk"]
	if !ok || len(jwkRaw) == 0 {
		return nil, nil, errors.New("jwk field not found")
	}

	var jwksJSON strings.Builder
	jwksJSON.WriteString(`{"keys":[`)
	jwksJSON.Write(jwkRaw)
	jwksJSON.WriteString(`]}`)
	ks, err := jwt.JWKSetToPublicKeysetHandle([]byte(jwksJSON.String()))
	if err != nil {
		return nil, nil, fmt.Errorf("converting jwks to public keyset: %w", err)
	}
	if ks.Len() != 1 {
		return nil, nil, errors.New("jwks must contain exactly one key")
	}

	verifier, err := jwt.NewVerifier(ks)
	if err != nil {
		return nil, nil, fmt.Errorf("creating verifier: %w", err)
	}

	validator, err := dbscValidator()
	if err != nil {
		return nil, nil, err
	}

	verified, err := verifier.VerifyAndDecode(compact, validator)
	if err != nil {
		return nil, nil, fmt.Errorf("verifying and decoding: %w", err)
	}

	return verified, ks, nil
}

// verifyDBSCRegistrationProofAndJWKS verifies a registration proof and returns
// the JWKS JSON produced by Tink for the verified public key (for later refresh verification).
func verifyDBSCRegistrationProofAndJWKS(compactToken, expectedChallenge string) ([]byte, error) {
	compact := stripDBSCProofToken(compactToken)
	if compact == "" {
		return nil, errors.New("empty proof token")
	}
	v, kh, err := parseDBSCRegistrationProof(compact)
	if err != nil {
		return nil, err
	}
	if !v.HasJWTID() {
		return nil, errors.New("missing jti claim")
	}
	jti, err := v.JWTID()
	if err != nil {
		return nil, fmt.Errorf("jti claim: %w", err)
	}
	if jti != expectedChallenge {
		return nil, errors.New("jti does not match registration challenge")
	}
	return jwt.JWKSetFromPublicKeysetHandle(kh)
}

// parseDBSCRefreshProof verifies a refresh DBSC proof using the stored public
// keyset (JWKS from registration).
func parseDBSCRefreshProof(compact string, handle *keyset.Handle) (*jwt.VerifiedJWT, error) {
	verifier, err := jwt.NewVerifier(handle)
	if err != nil {
		return nil, fmt.Errorf("creating verifier: %w", err)
	}

	validator, err := dbscValidator()
	if err != nil {
		return nil, err
	}

	verified, err := verifier.VerifyAndDecode(compact, validator)
	if err != nil {
		return nil, fmt.Errorf("verifying and decoding: %w", err)
	}

	return verified, nil
}

// verifyDBSCResponse validates Secure-Session-Response for a refresh-style
// proof: signature with the bound key in jwksJSON, typ dbsc+jwt, and jti equal
// to the active challenge (§9.10).
func verifyDBSCResponse(raw string, jwksJSON []byte, challenge string) error {
	if challenge == "" {
		return errors.New("no active challenge")
	}
	token := stripDBSCProofToken(raw)
	if token == "" {
		return errors.New("empty proof token")
	}
	ks, err := jwt.JWKSetToPublicKeysetHandle(jwksJSON)
	if err != nil {
		return fmt.Errorf("jwks to public keyset: %w", err)
	}
	verified, err := parseDBSCRefreshProof(token, ks)
	if err != nil {
		return err
	}
	if !verified.HasJWTID() {
		return errors.New("missing jti claim")
	}
	jti, err := verified.JWTID()
	if err != nil {
		return fmt.Errorf("jti claim: %w", err)
	}
	if jti != challenge {
		return errors.New("jti does not match challenge")
	}
	return nil
}

// parseToken parses a JWT token string into its three parts: header, claims,
// and signature. Returns ok=false if the token format is invalid (must have
// exactly 2 periods).
func parseToken(s string) (header, claims, sig string, ok bool) {
	header, s, ok = strings.Cut(s, ".")
	if !ok { // no period found
		return "", "", "", false
	}
	claims, s, ok = strings.Cut(s, ".")
	if !ok { // only one period found
		return "", "", "", false
	}
	sig, _, ok = strings.Cut(s, ".")
	if ok { // three periods found (more than expected)
		return "", "", "", false
	}
	return header, claims, sig, true
}
