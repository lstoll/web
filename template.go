package web

import (
	"html/template"
	"net/http"

	"github.com/lstoll/web/csp"
	"github.com/lstoll/web/session"
)

// BaseFuncMap is a set of placeholder functions to use at parse time. These
// will be replaced dynamically with the proper versions at template execution
// time.
var BaseFuncMap = template.FuncMap{
	// CSRFField writes out a HTML input field for the CSRF token
	// Deprecated: CSRF protection is now handled automatically by Sec-Fetch headers
	"CSRFField": func() template.HTML { panic("base func map should not be used") },
	// CSRFToken returns the bare CSRF token, to manually construct submissions
	// Deprecated: CSRF protection is now handled automatically by Sec-Fetch headers
	"CSRFToken": func() string { panic("base func map should not be used") },
	// HasFlash indicates if there is a flash value
	"HasFlash": func() bool { panic("base func map should not be used") },
	// FlashIsError indicates if the flash message is an error. If not, it is
	// informational.
	"FlashIsError": func() bool { panic("base func map should not be used") },
	// FlashMessage returns the current flash message, and removes it from the
	// session.
	"FlashMessage": func() string { panic("base func map should not be used") },
	// StaticPath constructs a full, cachable path for the file in the static
	// store.
	"StaticPath": func(string) (string, error) { panic("base func map should not be used") },
	// ScriptNonceAttr returns the HTML attribute for the script nonce if it's
	// in use, otherwise an empty value.
	"ScriptNonceAttr": func(string) (string, error) { panic("base func map should not be used") },
	// StyleNonceAttr returns the HTML attribute for the style nonce if it's
	// in use, otherwise an empty value.
	"StyleNonceAttr": func(string) (string, error) { panic("base func map should not be used") },
}

func TemplateFuncs(r *http.Request, addlFuncs template.FuncMap) template.FuncMap {
	sess := session.FromContext(r.Context())
	fm := map[string]any{
		"HasFlash": func() bool {
			return sess.HasFlash()
		},
		"FlashIsError": func() bool {
			return sess.FlashIsError()
		},
		"FlashMessage": func() string {
			return sess.FlashMessage()
		},
		"StaticPath": func(p string) (string, error) {
			return staticHandlerFromContext(r.Context()).PathFor(p)
		},
		"ScriptNonceAttr": func() template.HTMLAttr {
			nonce, ok := csp.GetScriptNonce(r.Context())
			if !ok {
				return ""
			}
			return template.HTMLAttr(`nonce="` + nonce + `"`)
		},
		"StyleNonceAttr": func() template.HTMLAttr {
			nonce, ok := csp.GetStyleNonce(r.Context())
			if !ok {
				return ""
			}
			return template.HTMLAttr(`nonce="` + nonce + `"`)
		},
	}
	for k, v := range addlFuncs {
		fm[k] = v
	}
	return fm
}
