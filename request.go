package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/lstoll/web/form"
	"github.com/lstoll/web/session"
)

type Request struct {
	r *http.Request
}

// Session returns the session associated with this request.
func (b *Request) Session() session.Session {
	return session.MustFromContext(b.r.Context())
}

func (b *Request) PostForm() url.Values {
	return b.r.PostForm
}

func (b *Request) Cookie(name string) (*http.Cookie, error) {
	return b.r.Cookie(name)
}

func (b *Request) URL() *url.URL {
	return b.r.URL
}

func (b *Request) PathValue(name string) string {
	return b.r.PathValue(name)
}

func (b *Request) UnmarshalJSONBody(target any) error {
	if !isJSONContentType(b.r.Header.Get("content-type")) {
		return fmt.Errorf("can not unmarshal non-json content type %s body", b.r.Header.Get("content-type"))
	}
	if err := json.NewDecoder(b.r.Body).Decode(&target); err != nil {
		return err
	}
	return nil
}

// DecodeForm unpacks the POST form into the target. The target type should be
// tagged appropriately.
func (b *Request) DecodeForm(target any) error {
	if !strings.HasPrefix(b.r.Header.Get("content-type"), "application/x-www-form-urlencoded") &&
		!strings.HasPrefix(b.r.Header.Get("content-type"), "multipart/form-data") {
		return fmt.Errorf("request is not form data")
	}

	if err := b.r.ParseForm(); err != nil {
		return fmt.Errorf("parsing request form: %w", err)
	}

	if err := form.Decode(b.r.PostForm, target); err != nil {
		return err
	}

	return nil
}

// RawRequest returns the raw http.Request underlying this request.
//
// Deprecated: This should only be used in exceptional circumstances. Prefer
// extending the BrowserRequest, or document why it is needed.
func (b *Request) RawRequest() *http.Request {
	return b.r
}
