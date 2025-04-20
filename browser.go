package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
)

type BrowserRequest struct {
	rw http.ResponseWriter
	r  *http.Request
}

func (b *BrowserRequest) PostForm() url.Values {
	return b.r.PostForm
}

func (b *BrowserRequest) Cookie(name string) (*http.Cookie, error) {
	return b.r.Cookie(name)
}

func (b *BrowserRequest) URL() *url.URL {
	return b.r.URL
}

func (b *BrowserRequest) PathValue(name string) string {
	return b.r.PathValue(name)
}

func (b *BrowserRequest) UnmarshalJSONBody(target any) error {
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
func (b *BrowserRequest) DecodeForm(target any) error {
	if !strings.HasPrefix(b.r.Header.Get("content-type"), "application/x-www-form-urlencoded") &&
		!strings.HasPrefix(b.r.Header.Get("content-type"), "multipart/form-data") {
		return fmt.Errorf("request is not form data")
	}

	if err := b.r.ParseForm(); err != nil {
		return fmt.Errorf("parsing request form: %w", err)
	}

	if err := decodeForm(b.r.PostForm, target); err != nil {
		return err
	}

	return nil
}

// RawRequest returns the raw http.Request underlying this request.
//
// Deprecated: This should only be used in exceptional circumstances. Prefer
// extending the BrowserRequest, or document why it is needed.
func (b *BrowserRequest) RawRequest() *http.Request {
	return b.r
}

// RawResponseWriter returns the raw http.ResponseWriter underlying this
// request. If this is written to, a NilResponse should be returned from the
// handler.
//
// Deprecated: This should only be used in exceptional circumstances. Prefer
// extending the BrowserRequest, or document why it is needed.
func (b *BrowserRequest) RawResponseWriter() http.ResponseWriter {
	return b.rw
}

type BrowserResponse interface {
	isBrowserResponse()
	getSettableCookies() []*http.Cookie
}

type CommonResponse struct {
	Cookies []*http.Cookie
}

func (c *CommonResponse) getSettableCookies() []*http.Cookie {
	return c.Cookies
}

func (*CommonResponse) isBrowserResponse() {}

// NilResponse indicates that no action should be taken. This should be used if
// the response was handled directly.
type NilResponse struct {
	CommonResponse
}

type TemplateResponse struct {
	CommonResponse
	Name string
	// Funcs are additional functions merged in to the rendered template.
	Funcs template.FuncMap
	// Templates to render response from. If not set, the configured templates
	// on the server are used.
	Templates *template.Template
	Data      any
}

type JSONResponse struct {
	CommonResponse
	// Data to be marshaled to JSON
	Data any
}

type RedirectResponse struct {
	CommonResponse
	// Code for redirect. If not set, http.StatusSeeOther(303) will be used
	Code int
	// URL to redirect to
	URL string
}

func isJSONContentType(contentType string) bool {
	if contentType == "" {
		return false
	}

	contentType = strings.ToLower(contentType)
	if strings.HasPrefix(contentType, "application/json") {
		return true
	}

	if strings.HasSuffix(contentType, "+json") && strings.HasPrefix(contentType, "application/") {
		return true
	}

	return false
}
