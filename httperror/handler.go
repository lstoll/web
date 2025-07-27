package httperror

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
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

	slog.ErrorContext(r.Context(), "error in web handler", "err", err, "path", r.URL.Path)

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
		http.Error(w, http.StatusText(he.Code()), he.Code())
		return
	}

	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
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
					stack := debug.Stack()

					switch v := p.(type) {
					case error:
						err = fmt.Errorf("panic recovered: %v", v)
					default:
						err = fmt.Errorf("panic recovered: %v", v)
					}

					// Log the panic with stack trace
					slog.ErrorContext(r.Context(), "panic recovered in web handler",
						"panic", p,
						"path", r.URL.Path,
						"stack", string(stack))

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
