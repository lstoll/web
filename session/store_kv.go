package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var DefaultKVStoreCookieOpts = &CookieOpts{
	Name: "__Host-session-id",
	Path: "/",
}

type KV interface {
	Get(_ context.Context, key string) (_ []byte, found bool, _ error)
	Set(_ context.Context, key string, expiresAt time.Time, value []byte) error
	Delete(_ context.Context, key string) error
}

var _ Store = (*KVStore)(nil)

type KVStore struct {
	kv         KV
	cookieOpts *CookieOpts
}

type KVStoreOpts struct {
	CookieOpts *CookieOpts
}

func NewKVStore(kv KV, opts *KVStoreOpts) (*KVStore, error) {
	s := &KVStore{
		kv:         kv,
		cookieOpts: DefaultKVStoreCookieOpts,
	}
	if opts != nil {
		if opts.CookieOpts != nil {
			s.cookieOpts = opts.CookieOpts
		}
	}
	return s, nil
}

// GetSession loads and unmarshals the session in to into
func (k *KVStore) GetSession(r *http.Request) ([]byte, error) {
	kvSess := k.getOrInitKVSess(r)

	// TODO(lstoll) differentiate deleted vs. emptied

	if kvSess.id == "" {
		// no active session loaded, try and fetch from cookie
		cookie, err := r.Cookie(k.cookieOpts.Name)
		if err != nil {
			if errors.Is(err, http.ErrNoCookie) {
				// kvSess.id = newSID()
				return nil, nil
			}
			return nil, fmt.Errorf("getting cookie %s: %w", k.cookieOpts.Name, err)
		}
		kvSess.id = cookie.Value
	}

	b, ok, err := k.kv.Get(r.Context(), k.storeID(kvSess.id))
	if err != nil {
		return nil, fmt.Errorf("loading from KV: %w", err)
	}
	if !ok {
		return nil, nil
	}

	return b, nil
}

// PutSession saves a session. If a session exists it should be updated, otherwise
// a new session should be created.
func (k *KVStore) PutSession(w http.ResponseWriter, r *http.Request, expiresAt time.Time, data []byte) error {
	kvSess := k.getOrInitKVSess(r)
	if kvSess.id == "" {
		kvSess.id = newSID()
	}

	if err := k.kv.Set(r.Context(), k.storeID(kvSess.id), expiresAt, data); err != nil {
		return fmt.Errorf("putting session data: %w", err)
	}

	c := k.cookieOpts.newCookie(expiresAt)
	c.Expires = expiresAt
	c.Value = kvSess.id

	removeCookieByName(w, c.Name)
	http.SetCookie(w, c)

	return nil
}

// DeleteSession deletes the session.
func (k *KVStore) DeleteSession(w http.ResponseWriter, r *http.Request) error {
	kvSess := k.getOrInitKVSess(r)
	if kvSess.id == "" {
		// no session ID to delete
		return nil
	}

	if err := k.kv.Delete(r.Context(), k.storeID(kvSess.id)); err != nil {
		return fmt.Errorf("deleting session %s from store: %w", kvSess.id, err)
	}

	// always clear the cookie
	dc := k.cookieOpts.newCookie(time.Time{})
	dc.MaxAge = -1

	removeCookieByName(w, dc.Name)
	http.SetCookie(w, dc)

	// assign a fresh SID, so if we do save again it'll go under a new session.
	// If not, it's ignored. This prevents a `Get` from trying to re-load from
	// the cookie.
	kvSess.id = newSID()

	return nil
}

func (k *KVStore) getOrInitKVSess(r *http.Request) *kvSession {
	kvSess, ok := r.Context().Value(kvSessCtxKey{inst: k}).(*kvSession)
	if ok {
		return kvSess
	}

	kvSess = &kvSession{}
	*r = *r.WithContext(context.WithValue(r.Context(), kvSessCtxKey{inst: k}, kvSess))

	return kvSess
}

func (k *KVStore) storeID(id string) string {
	h := sha256.New()
	h.Write([]byte(id))
	return hex.EncodeToString(h.Sum(nil))
}

type kvSessCtxKey struct{ inst *KVStore }

// kvSession tracks information about the session across the request's context
type kvSession struct {
	id string
}

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
