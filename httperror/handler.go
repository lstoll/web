package httperror

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/lstoll/web/internal"
)

// ErrorHandler defines the interface for handling errors
type ErrorHandler interface {
	HandleError(w http.ResponseWriter, r *http.Request, err error)
}

type ErrorHandlerFunc func(w http.ResponseWriter, r *http.Request, err error)

func (f ErrorHandlerFunc) HandleError(w http.ResponseWriter, r *http.Request, err error) {
	f(w, r, err)
}

// DefaultErrorHandler provides a basic implementation of ErrorHandler
func DefaultErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	var he HTTPError
	isHttpError := errors.As(err, &he)

	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		code := http.StatusInternalServerError
		errMsg := http.StatusText(http.StatusInternalServerError)

		if isHttpError {
			code = he.Code()
			errMsg = he.Error()
		} else {
			slog.Error("internal error in web handler", "err", err, "path", r.URL.Path)
		}

		w.WriteHeader(code)
		jsonErr := struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}{}
		jsonErr.Error.Code = code
		jsonErr.Error.Message = errMsg

		if err := json.NewEncoder(w).Encode(jsonErr); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	if isHttpError {
		http.Error(w, he.Error(), he.Code())
		return
	}

	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

var _ internal.UnwrappableResponseWriter = (*responseWriter)(nil)

// responseWriter wraps an http.ResponseWriter to intercept error responses
type responseWriter struct {
	http.ResponseWriter
	err           error
	code          int
	headerWritten bool
	buffer        bytes.Buffer
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		code:           http.StatusOK,
	}
}

func (w *responseWriter) WriteHeader(code int) {
	w.code = code

	if code < 400 {
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *responseWriter) Write(p []byte) (int, error) {
	if w.code >= 400 {
		return w.buffer.Write(p)
	}
	return w.ResponseWriter.Write(p)
}

func (w *responseWriter) WriteError(err error) {
	w.err = err
}

func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Handler provides HTTP error handling middleware
type Handler struct {
	ErrorHandler ErrorHandler
	// RecoverPanic causes panics in wrapped handler to be recovered, and
	// reported as errors.
	RecoverPanic bool
}

// Handle wraps an http.Handler to provide centralized error handling
func (h *Handler) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := newResponseWriter(w)
		ctx := r.Context()

		defer func() {
			if h.RecoverPanic {
				if p := recover(); p != nil {
					var err error
					switch v := p.(type) {
					case error:
						err = fmt.Errorf("panic recovered: %w", v)
					default:
						err = fmt.Errorf("panic recovered: %v", v)
					}

					if h.ErrorHandler != nil {
						h.ErrorHandler.HandleError(w, r, err)
					} else {
						DefaultErrorHandler(w, r, err)
					}
					return
				}
			}

			if rw.err != nil {
				if h.ErrorHandler != nil {
					h.ErrorHandler.HandleError(w, r, rw.err)
				} else {
					DefaultErrorHandler(w, r, rw.err)
				}
			} else if rw.code >= 400 {
				err := New(rw.code, rw.buffer.String())
				if h.ErrorHandler != nil {
					h.ErrorHandler.HandleError(w, r, err)
				} else {
					DefaultErrorHandler(w, r, err)
				}
			}
		}()

		next.ServeHTTP(rw, r.WithContext(ctx))
	})
}
