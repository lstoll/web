package web

import (
	"html/template"
	"net/http"

	"maps"

	"github.com/lstoll/web/csp"
	"github.com/lstoll/web/session"
)

func TemplateFuncs(r *http.Request, addlFuncs template.FuncMap) template.FuncMap {
	sess, sessOk := session.FromContext(r.Context())
	sh, shOk := staticHandlerFromContext(r.Context())

	fm := map[string]any{
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

	if sessOk {
		fm["HasFlash"] = sess.HasFlash
		fm["FlashIsError"] = sess.FlashIsError
		fm["FlashMessage"] = sess.FlashMessage
	}

	if shOk {
		fm["StaticPath"] = sh.PathFor
	}

	maps.Copy(fm, addlFuncs)

	return fm
}
