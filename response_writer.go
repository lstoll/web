package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/lstoll/web/internal"
)

// newResponseWriter creates a new ResponseWriter
func newResponseWriter(w http.ResponseWriter) ResponseWriter {
	return &responseWriter{
		ResponseWriter: w,
	}
}

type ResponseWriter interface {
	http.ResponseWriter
	WriteResponse(r *Request, resp BrowserResponse) error
}

var (
	_ ResponseWriter                     = (*responseWriter)(nil)
	_ internal.UnwrappableResponseWriter = (*responseWriter)(nil)
)

type responseWriter struct {
	http.ResponseWriter
	handled bool
}

func (w *responseWriter) WriteResponse(r *Request, resp BrowserResponse) error {
	if w.handled {
		return fmt.Errorf("response already written")
	}
	w.handled = true

	// Set any cookies from the response
	for _, c := range resp.getSettableCookies() {
		http.SetCookie(w, c)
	}

	// Handle different response types
	switch resp := resp.(type) {
	case *TemplateResponse:
		return w.writeTemplateResponse(r, resp)
	case *JSONResponse:
		return w.writeJSONResponse(resp)
	case *NilResponse:
		// Do nothing, should be handled already
		return nil
	case *RedirectResponse:
		return w.writeRedirectResponse(r, resp)
	default:
		return fmt.Errorf("unhandled browser response type: %T", resp)
	}
}

func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *responseWriter) writeTemplateResponse(req *Request, resp *TemplateResponse) error {
	t := resp.Templates.Funcs(TemplateFuncs(req.r.Context(), resp.Funcs))

	// Buffer the render to capture errors before writing
	var buf bytes.Buffer
	err := t.ExecuteTemplate(&buf, resp.Name, resp.Data)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, &buf)
	return err
}

func (w *responseWriter) writeJSONResponse(resp *JSONResponse) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(resp.Data)
}

func (w *responseWriter) writeRedirectResponse(req *Request, resp *RedirectResponse) error {
	code := resp.Code
	if code == 0 {
		code = http.StatusSeeOther
	}
	http.Redirect(w, req.r, resp.URL, code)
	return nil
}
