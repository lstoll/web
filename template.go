package web

import (
	"context"
	"fmt"
	"html/template"

	"maps"

	"lds.li/web/csp"
	"lds.li/web/internal/ctxkeys"
	"lds.li/web/session"
)

func TemplateFuncs(ctx context.Context, addlFuncs template.FuncMap) template.FuncMap {
	sess, sessOk := session.FromContext(ctx)
	sh, shOk := ctxkeys.StaticHandlerFromContext(ctx)

	fm := map[string]any{
		// CSP
		"ScriptNonceAttr": func() template.HTMLAttr {
			nonce, ok := csp.GetScriptNonce(ctx)
			if !ok {
				return ""
			}
			return template.HTMLAttr(`nonce="` + nonce + `"`)
		},
		"StyleNonceAttr": func() template.HTMLAttr {
			nonce, ok := csp.GetStyleNonce(ctx)
			if !ok {
				return ""
			}
			return template.HTMLAttr(`nonce="` + nonce + `"`)
		},
		// Session
		"HasFlash": func() (bool, error) {
			if !sessOk {
				return false, fmt.Errorf("session not found")
			}
			return sess.HasFlash(), nil
		},
		"FlashIsError": func() (bool, error) {
			if !sessOk {
				return false, fmt.Errorf("session not found")
			}
			return sess.FlashIsError(), nil
		},
		"FlashMessage": func() (string, error) {
			if !sessOk {
				return "", fmt.Errorf("session not found")
			}
			return sess.FlashMessage(), nil
		},
		// Static
		"StaticPath": func(file string) (string, error) {
			if !shOk {
				return "", fmt.Errorf("static handler not found")
			}
			return sh.PathFor(file)
		},
	}

	maps.Copy(fm, addlFuncs)

	return fm
}
