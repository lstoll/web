package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// contextKey is used for context.WithValue keys
type contextKey string

// serverContextKey is used to store the server in the context
const serverContextKey contextKey = "server"

func WithServer(ctx context.Context, server *Server) context.Context {
	return context.WithValue(ctx, serverContextKey, server)
}

// NewResponseWriter creates a new ResponseWriter
func NewResponseWriter(w http.ResponseWriter, r *http.Request, s *Server) ResponseWriter {
	return &responseWriter{
		rw:     w,
		r:      r,
		server: s,
	}
}

type ResponseWriter interface {
	WriteError(err error) error
	WriteResponse(resp BrowserResponse) error
	HTTPResponseWriter() http.ResponseWriter
}

var _ ResponseWriter = (*responseWriter)(nil)

type responseWriter struct {
	rw      http.ResponseWriter
	r       *http.Request
	server  *Server
	handled bool
}

func (w *responseWriter) WriteResponse(resp BrowserResponse) error {
	if w.handled {
		return fmt.Errorf("response already written")
	}
	w.handled = true

	// Set any cookies from the response
	for _, c := range resp.getSettableCookies() {
		http.SetCookie(w.rw, c)
	}

	// Handle different response types
	switch resp := resp.(type) {
	case *TemplateResponse:
		return w.writeTemplateResponse(resp)
	case *JSONResponse:
		return w.writeJSONResponse(resp)
	case *NilResponse:
		// Do nothing, should be handled already
		return nil
	case *RedirectResponse:
		return w.writeRedirectResponse(resp)
	default:
		return fmt.Errorf("unhandled browser response type: %T", resp)
	}
}

func (w *responseWriter) writeTemplateResponse(resp *TemplateResponse) error {
	server := w.serverFromContext()
	if server == nil {
		return fmt.Errorf("server not found in context")
	}

	templates := resp.Templates
	if templates == nil {
		templates = server.config.Templates
	}

	t := templates.Funcs(server.buildFuncMap(w.r, resp.Funcs))

	// Buffer the render to capture errors before writing
	var buf bytes.Buffer
	err := t.ExecuteTemplate(&buf, resp.Name, resp.Data)
	if err != nil {
		return err
	}

	_, err = io.Copy(w.rw, &buf)
	return err
}

func (w *responseWriter) writeJSONResponse(resp *JSONResponse) error {
	w.rw.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w.rw).Encode(resp.Data)
}

func (w *responseWriter) writeRedirectResponse(resp *RedirectResponse) error {
	code := resp.Code
	if code == 0 {
		code = http.StatusSeeOther
	}
	http.Redirect(w.rw, w.r, resp.URL, code)
	return nil
}

func (w *responseWriter) WriteError(err error) error {
	if w.handled {
		return fmt.Errorf("response already written")
	}
	w.handled = true

	server := w.serverFromContext()
	if server == nil {
		// Fallback to a basic error handler if we can't get the server
		http.Error(w.rw, err.Error(), http.StatusInternalServerError)
		return nil
	}

	server.config.ErrorHandler(w.rw, w.r, server.config.Templates.Funcs(server.buildFuncMap(w.r, nil)), err)
	return nil
}

func (w *responseWriter) HTTPResponseWriter() http.ResponseWriter {
	return w.rw
}

func (w *responseWriter) serverFromContext() *Server {
	if w.server != nil {
		return w.server
	}

	// Get server from context if passed that way
	if srv, ok := w.r.Context().Value(serverContextKey).(*Server); ok {
		return srv
	}

	return nil
}
