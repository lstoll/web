package httperror

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lstoll/web/internal"
)

func TestHTTPError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
		wantMsg  string
	}{
		{
			name:     "bad request error",
			err:      BadRequestErrf("invalid input: %s", "missing field"),
			wantCode: http.StatusBadRequest,
			wantMsg:  "http error 400: invalid input: missing field",
		},
		{
			name:     "forbidden error",
			err:      ForbiddenErrf("access denied: %s", "insufficient permissions"),
			wantCode: http.StatusForbidden,
			wantMsg:  "http error 403: access denied: insufficient permissions",
		},
		{
			name:     "custom error",
			err:      Newf(http.StatusTeapot, "I'm a teapot"),
			wantCode: http.StatusTeapot,
			wantMsg:  "http error 418: I'm a teapot",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			he, ok := tt.err.(HTTPError)
			if !ok {
				t.Fatalf("error is not an HTTPError: %T", tt.err)
			}

			if got := he.Code(); got != tt.wantCode {
				t.Errorf("Code() = %v, want %v", got, tt.wantCode)
			}

			if got := he.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %v, want %v", got, tt.wantMsg)
			}
		})
	}
}

func TestHandler(t *testing.T) {
	tests := []struct {
		name         string
		handler      http.Handler
		errorHandler ErrorHandler
		recoverPanic bool
		wantCode     int
		wantBody     string
		shouldPanic  bool
		panicValue   any
	}{
		{
			name: "success response",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			}),
			wantCode: http.StatusOK,
			wantBody: "success",
		},
		{
			name: "error response",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "not found", http.StatusNotFound)
			}),
			wantCode: http.StatusNotFound,
			wantBody: "http error 404: not found\n\n",
		},
		{
			name: "custom error handler",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "bad request", http.StatusBadRequest)
			}),
			errorHandler: ErrorHandlerFunc(func(w http.ResponseWriter, r *http.Request, err error) {
				w.WriteHeader(http.StatusTeapot)
				w.Write([]byte("custom error handler"))
			}),
			wantCode: http.StatusTeapot,
			wantBody: "custom error handler",
		},
		{
			name: "panic with recovery",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic("test panic")
			}),
			recoverPanic: true,
			wantCode:     http.StatusInternalServerError,
			wantBody:     "Internal Server Error\n",
		},
		{
			name: "panic without recovery",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic("test panic")
			}),
			recoverPanic: false,
			shouldPanic:  true,
			panicValue:   "test panic",
		},
		{
			name: "httperror.ResponseWriter",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				erw := w.(ResponseWriter)
				erw.WriteError(New(http.StatusUnauthorized, "Unauthorized"))
			}),
			wantCode: http.StatusUnauthorized,
			wantBody: "http error 401: Unauthorized\n",
		},
		{
			name: "httperror.ResponseWriter wrapped",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// provided mostly as an example of how this works, for
				// consumers of this package.
				erw, ok := internal.UnwrapResponseWriterTo[ResponseWriter](&wrapRW{w})
				if !ok {
					panic("httperror.ResponseWriter not found")
				}
				erw.WriteError(New(http.StatusUnauthorized, "Unauthorized"))
			}),
			wantCode: http.StatusUnauthorized,
			wantBody: "http error 401: Unauthorized\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{
				ErrorHandler: tt.errorHandler,
				RecoverPanic: tt.recoverPanic,
			}

			req := httptest.NewRequest("GET", "/", nil)
			rec := httptest.NewRecorder()

			if tt.shouldPanic {
				defer func() {
					if r := recover(); r != tt.panicValue {
						t.Errorf("panic value = %v, want %v", r, tt.panicValue)
					}
				}()
			}

			h.Handle(tt.handler).ServeHTTP(rec, req)

			if !tt.shouldPanic {
				if rec.Code != tt.wantCode {
					t.Errorf("status code = %v, want %v", rec.Code, tt.wantCode)
				}
				if rec.Body.String() != tt.wantBody {
					t.Errorf("body = %v, want %v", rec.Body.String(), tt.wantBody)
				}
			}
		})
	}
}

type wrapRW struct {
	http.ResponseWriter
}

func (w *wrapRW) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
