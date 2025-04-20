package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/justinas/nosurf"
	"github.com/lstoll/web/cors"
	"github.com/lstoll/web/csp"
	"github.com/lstoll/web/requestid"
	"github.com/lstoll/web/session"
)

const staticPrefix = "/static/"

// Session defines the interface we expect a session object to have.
type Session interface {
	// HasFlash indicates if there is a flash message
	HasFlash() bool
	// FlashIsError indicates that the flash message is an error. If not, info
	// is assumed.
	FlashIsError() bool
	// FlashMessage returns the current flash message. The flash should be
	// cleared when this is called, and the session will be saved after this.
	FlashMessage() string
}

var DefaultCSPOpts = []csp.HandlerOpt{
	csp.DefaultSrc(`'none'`),
	csp.ScriptSrc(`'self' 'unsafe-inline'`),
	csp.StyleSrc(`'self' 'unsafe-inline'`),
	csp.ImgSrc(`'self'`),
	csp.ConnectSrc(`'self'`),
	csp.FontSrc(`'self'`),
	csp.BaseURI(`'self'`),
	csp.FrameAncestors(`'none'`),
}

type Config[SessT Session] struct {
	BaseURL        *url.URL
	SessionManager *session.Manager[SessT]
	ErrorHandler   func(w http.ResponseWriter, r *http.Request, templates *template.Template, err error)
	Templates      *template.Template
	// TemplateFuncs are additional functions merged in to all template
	// invocations. For each request, this will be called with the request's
	// context, and the returned functions merged in.
	TemplateFuncs func(ctx context.Context) template.FuncMap
	Static        fs.FS
	CSPOpts       []csp.HandlerOpt
	// ScriptNonce indicates that a nonce should be used for inline scripts.
	// This will update the CSP, and the template func will return a value.
	ScriptNonce bool
	// ScriptNonce indicates that a nonce should be used for inline styles. This
	// will update the CSP, and the template func will return a value.
	StyleNonce bool
	// Mux that handlers will mount on. If nil, a new mux will be created. If an
	// existing mux is passed, a handler for the static content will be added.
	Mux *http.ServeMux
	// BrowserAuthMiddleware will be injected in to all Browser handled pages,
	// to manage their auth.
	BrowserAuthMiddleware func(h http.Handler, opts ...HandleOpt) BrowserHandler
}

func NewServer[SessT Session](c *Config[SessT]) (*Server[SessT], error) {
	if c.CSPOpts == nil {
		c.CSPOpts = DefaultCSPOpts
	}
	var mountStatic = true
	if c.Mux == nil {
		c.Mux = http.NewServeMux()
		mountStatic = false
	}
	if c.ErrorHandler == nil {
		c.ErrorHandler = DefaultErrorHandler
	}

	sh, err := newStaticFileHandler(c.Static, staticPrefix)
	if err != nil {
		return nil, fmt.Errorf("creating static handler: %w", err)
	}

	svr := &Server[SessT]{
		config:        c,
		staticHandler: sh,
		mux:           c.Mux,
	}

	if mountStatic {
		c.Mux.Handle("/static/", svr)
	}

	return svr, nil
}

type Server[SessT Session] struct {
	config        *Config[SessT]
	mux           *http.ServeMux
	staticHandler *staticFileHandler
}

func (s *Server[SessT]) SetAuthMiddleware(m func(h http.Handler, opts ...HandleOpt) BrowserHandler) {
	s.config.BrowserAuthMiddleware = m
}

func (s *Server[SessT]) Session() *session.Manager[SessT] {
	return s.config.SessionManager
}

func (s *Server[SessT]) HandleRaw(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, s.baseWrappers(handler))
}

type HandleBrowserOpts struct {
	SkipCSRF bool
}

type HandleOpt interface{}

// HandleSkipCSRF is an opt that can be passed to a HandleBrowser to skip CSRF
// protection.
//
// Deprecated: CSRF should always be passed via form or header for relevant
// actions. Exceptions should be documented.
type HandleSkipCSRF struct{}

// HandleBrowser mounts a handler targeted at browser users at the given path
func (s *Server[SessT]) HandleBrowser(pattern string, h http.Handler, opts ...HandleOpt) {
	csrfh := nosurf.New(h)
	csrfh.ExemptFunc(isRequestCSRFExempt)
	csrfh.SetFailureHandler(http.HandlerFunc(s.csrfFailureHandler))
	if slices.ContainsFunc(opts, func(o HandleOpt) bool {
		_, ok := o.(HandleSkipCSRF)
		return ok
	}) {
		h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(contextWithCSRFExempt(r.Context()))
			csrfh.ServeHTTP(w, r)
		})
	} else {
		h = csrfh
	}
	prevh := h
	h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.config.BrowserAuthMiddleware != nil {
			s.BrowserHandler(s.config.BrowserAuthMiddleware(prevh, opts...)).ServeHTTP(w, r)
		} else {
			prevh.ServeHTTP(w, r)
		}
	})
	h = s.config.SessionManager.Wrap(h)
	h = s.cspHandler(h)
	h = cors.DenyPreflight(h)
	h = s.baseWrappers(h)
	s.mux.Handle(pattern, h)
}

func (s *Server[SessT]) cspHandler(wrap http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cspOpts := s.config.CSPOpts
		if s.config.ScriptNonce {
			nonce := rand.Text()
			cspOpts = append(cspOpts, csp.ScriptSrc("'nonce-"+nonce+"'"))
			r = r.WithContext(contextWithScriptNonce(r.Context(), nonce))
		}
		if s.config.StyleNonce {
			nonce := rand.Text()
			cspOpts = append(cspOpts, csp.StyleSrc("'nonce-"+nonce+"'"))
			r = r.WithContext(contextWithStyleNonce(r.Context(), nonce))
		}
		csp.NewHandler(*s.config.BaseURL, cspOpts...).Wrap(wrap).ServeHTTP(w, r)
	})
}

type BrowserHandler func(context.Context, *BrowserRequest) (BrowserResponse, error)

// BrowserHandler creates a new httpHandler from a higher-level abstraction,
// targeted towards responding to browsers.
func (s *Server[SessT]) BrowserHandler(h BrowserHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			s.config.ErrorHandler(w, r, s.config.Templates.Funcs(s.buildFuncMap(r, nil)), fmt.Errorf("parsing form: %w", err))
			return
		}
		br := &BrowserRequest{
			rw: w,
			r:  r,
		}

		resp, err := h(r.Context(), br)
		if err != nil {
			s.config.ErrorHandler(w, r, s.config.Templates.Funcs(s.buildFuncMap(r, nil)), err)
			return
		}

		for _, c := range resp.getSettableCookies() {
			http.SetCookie(w, c)
		}

		switch resp := resp.(type) {
		case *TemplateResponse:
			if resp.Templates == nil {
				resp.Templates = s.config.Templates
			}
			t := resp.Templates.Funcs(s.buildFuncMap(r, resp.Funcs))
			// we buffer the render, so we can capture errors better than
			// blowing up halfway through the write.
			var buf bytes.Buffer
			err := t.ExecuteTemplate(&buf, resp.Name, resp.Data)
			if err != nil {
				s.config.ErrorHandler(w, r, s.config.Templates.Funcs(s.buildFuncMap(r, nil)), err)
				return
			}
			if _, err := io.Copy(w, &buf); err != nil {
				s.config.ErrorHandler(w, r, s.config.Templates.Funcs(s.buildFuncMap(r, nil)), err)
				return
			}
		case *JSONResponse:
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp.Data); err != nil {
				s.config.ErrorHandler(w, r, s.config.Templates.Funcs(s.buildFuncMap(r, nil)), err)
				return
			}
		case *NilResponse:
			// do nothing, should be handled already
		case *RedirectResponse:
			code := resp.Code
			if code == 0 {
				code = http.StatusSeeOther
			}
			http.Redirect(w, r, resp.URL, code)
		default:
			panic(fmt.Sprintf("unhandled browser response type: %T", resp))
		}
	})
}

func (s *Server[SessT]) VerifyCSRFToken(r *BrowserRequest, token string) bool {
	return nosurf.VerifyToken(nosurf.Token(r.r), token)
}

func (s *Server[SessT]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, staticPrefix) {
		s.staticHandler.ServeHTTP(w, r)
		return
	}
	s.mux.ServeHTTP(w, r)
}

// baseWrappers installs the lowest-level handlers that all requests should
// have, like logging etc.
func (s *Server[SessT]) baseWrappers(h http.Handler) http.Handler {
	hh := loggingMiddleware(h)
	hh = requestid.Handler(true, hh)
	return hh
}

func (s *Server[SessT]) csrfFailureHandler(w http.ResponseWriter, r *http.Request) {
	s.config.ErrorHandler(w, r, s.config.Templates.Funcs(s.buildFuncMap(r, nil)), BadRequestErrf("CSRF validation failed"))
}

func DefaultErrorHandler(w http.ResponseWriter, r *http.Request, _ *template.Template, err error) {
	var forbiddenErr *ErrForbidden
	if errors.As(err, &forbiddenErr) {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	var badReqERR *ErrBadRequest
	if errors.As(err, &badReqERR) {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	slog.ErrorContext(r.Context(), "internal error in web handler", "err", err)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
