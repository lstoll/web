package session

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var defaultCookieStoreCookieOpts = &CookieOpts{
	Name: "__Host-session",
	Path: "/",
}

// CookieOpts can be used to customize the cookie used for tracking sessions.
type CookieOpts struct {
	Name     string
	Path     string
	Insecure bool
	Persist  bool
}

func (c *CookieOpts) newCookie(exp time.Time) *http.Cookie {
	hc := &http.Cookie{
		Name:     c.Name,
		Path:     c.Path,
		Secure:   !c.Insecure,
		HttpOnly: true,
	}
	if c.Persist {
		hc.MaxAge = int(time.Until(exp))
	}
	return hc
}

const (
	cookieMagic           = "EU1"
	compressedCookieMagic = "EC1"

	// compressThreshold is the size at which we decide to compress a cookie,
	// bytes
	compressThreshold = 512
	maxCookieSize     = 4096
)

var cookieValueEncoding = base64.RawURLEncoding

var _ Store = (*cookieStore)(nil)

type cookieStore struct {
	AEAD                AEAD
	cookieOpts          *CookieOpts
	CompressionDisabled bool
}

// GetSession loads and unmarshals the session in to into
func (c *cookieStore) GetSession(r *http.Request) ([]byte, error) {
	// no active session loaded, try and fetch from cookie
	cookie, err := r.Cookie(c.cookieOpts.Name)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			// no session, no op
			return nil, nil
		}
		return nil, fmt.Errorf("getting cookie %s: %w", c.cookieOpts.Name, err)
	}

	sp := strings.SplitN(cookie.Value, ".", 2)
	if len(sp) != 2 {
		return nil, errors.New("cookie does not contain two . separated parts")
	}
	magic := sp[0]
	cd, err := cookieValueEncoding.DecodeString(sp[1])
	if err != nil {
		return nil, fmt.Errorf("decoding cookie string: %w", err)
	}

	if magic != compressedCookieMagic && magic != cookieMagic {
		return nil, fmt.Errorf("cooking has bad magic prefix: %s", magic)
	}

	// uncompress if needed
	if magic == compressedCookieMagic {
		cr := getDecompressor()
		defer putDecompressor(cr)
		b, err := cr.Decompress(cd)
		if err != nil {
			return nil, fmt.Errorf("decompressing cookie: %w", err)
		}
		cd = b
	}

	// decrypt
	db, err := c.AEAD.Decrypt(cd, []byte(c.cookieOpts.Name))
	if err != nil {
		return nil, fmt.Errorf("decrypting cookie: %w", err)
	}

	expiresAt := time.Unix(int64(binary.LittleEndian.Uint64(db[:8])), 0)
	if expiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("cookie expired at %s", expiresAt)
	}
	db = db[8:]

	return db, nil
}

// PutSession saves a session. If a session exists it should be updated, otherwise
// a new session should be created.
func (c *cookieStore) PutSession(w http.ResponseWriter, r *http.Request, expiresAt time.Time, data []byte) error {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(expiresAt.Unix()))
	data = append(b, data...)

	magic := cookieMagic
	if len(data) > compressThreshold {
		cw := getCompressor()
		defer putCompressor(cw)

		b, err := cw.Compress(data)
		if err != nil {
			return fmt.Errorf("compressing cookie: %w", err)
		}
		data = b
		magic = compressedCookieMagic
	}

	var err error
	data, err = c.AEAD.Encrypt(data, []byte(c.cookieOpts.Name))
	if err != nil {
		return fmt.Errorf("encrypting cookie failed: %w", err)
	}

	cv := magic + "." + cookieValueEncoding.EncodeToString(data)
	if len(cv) > maxCookieSize {
		return fmt.Errorf("cookie size %d is greater than max %d", len(cv), maxCookieSize)
	}

	cookie := c.cookieOpts.newCookie(expiresAt)
	cookie.Value = cv
	http.SetCookie(w, cookie)

	return nil
}

// Delete deletes the session.
func (c *cookieStore) DeleteSession(w http.ResponseWriter, r *http.Request) error {
	dc := c.cookieOpts.newCookie(time.Time{})
	dc.MaxAge = -1
	http.SetCookie(w, dc)

	return nil
}
