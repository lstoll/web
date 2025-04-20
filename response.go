package web

import (
	"html/template"
	"net/http"
	"strings"
)

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
