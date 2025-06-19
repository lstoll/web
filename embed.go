package web

import (
	"context"
	"fmt"
	"html/template"
	"strings"
)

func SampleTemplateFunctions(smgr any) func(ctx context.Context) template.FuncMap {
	// TODO - it would be nice to have a way to pass the session manager and
	// other things in dynamically
	return func(ctx context.Context) template.FuncMap {
		return map[string]any{
			"UserLoggedIn": func() bool {
				// s := smgr.Get(ctx)
				// return s.Login.AuthenticatedFor("")
				return false
			},
			"CurrentUserName": func() string {
				// s := smgr.Get(ctx)
				// if s.Login != nil && s.Login.FullName != "" {
				// 	return s.Login.FullName
				// }
				return ""
			},
		}
	}
}

// WithSampleLayout takes the given template, and merges the layout template
// items in to it.
func WithSampleLayout(t *template.Template) (*template.Template, error) {
	// kept as a demo for merging templates
	templates := template.New("")
	for _, it := range templates.Templates() {
		if strings.HasPrefix(it.Name(), "_") {
			var err error
			t, err = t.AddParseTree(it.Name(), it.Tree)
			if err != nil {
				return nil, fmt.Errorf("adding template %s: %w", it.Name(), err)
			}
		}
	}
	return t, nil
}
