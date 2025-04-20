package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
)

type jsonErr struct {
	Error string `json:"error"`
}

var sampleTemplateFunctionsStub = template.FuncMap(map[string]any{
	"UserLoggedIn":    func() bool { panic("should not be called") },
	"CurrentUserName": func() string { panic("should not be called") },
})

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

// WithSamepleLayout takes the given template, and merges the layout template
// items in to it.
func WithSamepleLayout(t *template.Template) (*template.Template, error) {
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

func SampleErrorHandler(w http.ResponseWriter, r *http.Request, templates *template.Template, err error) {
	var (
		code    int
		title   string
		message string
	)

	var (
		forbiddenErr  *ErrForbidden
		badRequestErr *ErrBadRequest
	)
	if errors.As(err, &forbiddenErr) {
		slog.InfoContext(r.Context(), "Access denied error in web server", "err", err)
		code = http.StatusForbidden
		title = "Access Denied"
		message = "Access Denied"
	} else if errors.As(err, &badRequestErr) {
		slog.DebugContext(r.Context(), "Bad request in web server", "err", err)
		code = http.StatusBadRequest
		title = "Bad Request"
		message = "Bad Request"
	} else {
		slog.ErrorContext(r.Context(), "Internal error in web server", "err", err)
		code = http.StatusInternalServerError
		title = "Error"
		message = `An error has occurred.`
	}

	w.WriteHeader(code)

	if isJSONContentType(r.Header.Get("accept")) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(&jsonErr{
			Error: message,
		}); err != nil {
			slog.ErrorContext(r.Context(), "Error in error handler!", "err", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	_ = title
	if err := templates.ExecuteTemplate(w, "_error.tmpl.html", map[string]any{
		// "Base": BaseTemplateArgs{
		// 	Title: title,
		// },
		"ErrorMessage": template.HTML(message),
	}); err != nil {
		slog.ErrorContext(r.Context(), "Error in error handler!", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
