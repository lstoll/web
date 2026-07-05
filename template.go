package web

import (
	"context"
	"fmt"
	"html/template"

	"maps"

	"lds.li/web/csp"
	"lds.li/web/internal/ctxkeys"
)

func TemplateFuncs(ctx context.Context, addlFuncs template.FuncMap) template.FuncMap {
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
