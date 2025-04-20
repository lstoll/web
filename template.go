package web

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/justinas/nosurf"
)

// BaseFuncMap is a set of placeholder functions to use at parse time. These
// will be replaced dynamically with the proper versions at template execution
// time.
var BaseFuncMap = template.FuncMap{
	// CSRFField writes out a HTML input field for the CSRF token
	"CSRFField": func() template.HTML { panic("base func map should not be used") },
	// CSRFToken returns the bare CSRF token, to manually construct submissions
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

func (s *Server) buildFuncMap(r *http.Request, addlFuncs template.FuncMap) template.FuncMap {
	// sess := s.config.SessionManager.Get(r.Context())
	fm := map[string]any{
		"CSRFField": func() template.HTML {
			return template.HTML(fmt.Sprintf(`<input id="csrf_token" type="hidden" name="csrf_token" value="%s">`, nosurf.Token(r)))
		},
		"CSRFToken": func() string {
			return nosurf.Token(r)
		},
		"HasFlash": func() bool {
			// return sess.HasFlash() // TODO
			return false
		},
		"FlashIsError": func() bool {
			// return sess.FlashIsError() // TODO
			return false
		},
		"FlashMessage": func() string {
			// m := sess.FlashMessage()
			// s.config.SessionManager.Save(r.Context(), sess)
			// return m
			return ""
		},
		"StaticPath": func(p string) (string, error) {
			return s.staticHandler.URL(p)
		},
		"ScriptNonceAttr": func() template.HTMLAttr {
			nonce, ok := r.Context().Value(ctxKeyScriptNonce{}).(string)
			if !ok || nonce == "" {
				return ""
			}
			return template.HTMLAttr(`nonce="` + nonce + `"`)
		},
		"StyleNonceAttr": func() template.HTMLAttr {
			nonce, ok := r.Context().Value(ctxKeyStyleNonce{}).(string)
			if !ok || nonce == "" {
				return ""
			}
			return template.HTMLAttr(`nonce="` + nonce + `"`)
		},
	}
	if s.config.TemplateFuncs != nil {
		for k, v := range s.config.TemplateFuncs(r.Context()) {
			fm[k] = v
		}
	}
	for k, v := range addlFuncs {
		fm[k] = v
	}
	return fm
}
