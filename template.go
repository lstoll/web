package web

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"maps"

	"lds.li/web/csp"
	"lds.li/web/internal/ctxkeys"
)

// TemplateFuncStubs returns no-op template functions matching the names and
// signatures injected at render time by the framework. Register these when
// parsing templates so references like {{StaticPath "app.css"}} are valid at
// parse time; the framework overrides them when rendering TemplateResponse.
func TemplateFuncStubs() template.FuncMap {
	return maps.Clone(templateFuncStubs())
}

func templateFuncStubs() template.FuncMap {
	return template.FuncMap{
		"ScriptNonceAttr": func() template.HTMLAttr { return "" },
		"StyleNonceAttr":  func() template.HTMLAttr { return "" },
		"StaticPath":      func(string) (string, error) { return "", nil },
	}
}

// ParseTemplates parses templates from fsys with framework func stubs
// pre-registered.
func ParseTemplates(fsys fs.FS, patterns ...string) (*template.Template, error) {
	return template.New("").Funcs(TemplateFuncStubs()).ParseFS(fsys, patterns...)
}

func TemplateFuncs(ctx context.Context, addlFuncs template.FuncMap) template.FuncMap {
	sh, shOk := ctxkeys.StaticHandlerFromContext(ctx)

	fm := templateFuncStubs()
	fm["ScriptNonceAttr"] = func() template.HTMLAttr {
		nonce, ok := csp.GetScriptNonce(ctx)
		if !ok {
			return ""
		}
		return template.HTMLAttr(`nonce="` + nonce + `"`)
	}
	fm["StyleNonceAttr"] = func() template.HTMLAttr {
		nonce, ok := csp.GetStyleNonce(ctx)
		if !ok {
			return ""
		}
		return template.HTMLAttr(`nonce="` + nonce + `"`)
	}
	fm["StaticPath"] = func(file string) (string, error) {
		if !shOk {
			return "", fmt.Errorf("static handler not found")
		}
		return sh.PathFor(file)
	}

	maps.Copy(fm, addlFuncs)

	return fm
}
