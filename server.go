package web

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/lstoll/web/cors"
	"github.com/lstoll/web/csp"
	"github.com/lstoll/web/requestid"
	"github.com/lstoll/web/secfetch"
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

type Config struct {
	BaseURL        *url.URL
	SessionManager *session.Manager
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
	BrowserAuthMiddleware func(h http.Handler, opts ...HandleOpt) BrowserHandlerFunc
}

func NewServer(c *Config) (*Server, error) {
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

	svr := &Server{
		config:        c,
		staticHandler: sh,
		mux:           c.Mux,
	}

	if mountStatic {
		c.Mux.Handle("/static/", svr)
	}

	return svr, nil
}

type Server struct {
	config        *Config
	mux           *http.ServeMux
	staticHandler *staticFileHandler
}

func (s *Server) SetAuthMiddleware(m func(h http.Handler, opts ...HandleOpt) BrowserHandlerFunc) {
	s.config.BrowserAuthMiddleware = m
}

func (s *Server) Session() *session.Manager {
	return s.config.SessionManager
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
func (s *Server) HandleBrowser(pattern string, h http.Handler, opts ...HandleOpt) {
	// Check if we should skip CSRF protection
	skipCSRF := slices.ContainsFunc(opts, func(o HandleOpt) bool {
		_, ok := o.(HandleSkipCSRF)
		return ok
	})

	// Apply secfetch protection
	if skipCSRF {
		// If skipping CSRF, we'll still apply secfetch but allow cross-site requests
		h = secfetch.Protect(h, secfetch.AllowCrossSiteNavigation{}, secfetch.AllowCrossSiteAPI{})
	} else {
		// Standard protection
		h = secfetch.Protect(h)
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

func (s *Server) cspHandler(wrap http.Handler) http.Handler {
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

func (s *Server) HandleRaw(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, s.baseWrappers(handler))
}

func (s *Server) Handle(pattern string, h http.Handler, opts ...HandleOpt) {
	s.mux.Handle(pattern, s.buildBrowserMiddlewareStack(h, opts...))
}

func (s *Server) HandleFunc(pattern string, h func(w http.ResponseWriter, r *http.Request), opts ...HandleOpt) {
	s.Handle(pattern, http.HandlerFunc(h))
}

type BrowserHandlerFunc func(context.Context, ResponseWriter, *Request) error

func (s *Server) HandleBrowserFunc(pattern string, h BrowserHandlerFunc, opts ...HandleOpt) {
	hh := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create request with server in context
		ctx := WithServer(r.Context(), s)
		r = r.WithContext(ctx)

		// Create responsewriter and request
		brw := NewResponseWriter(w, r, s)
		br := &Request{rw: w, r: r}

		if err := h(ctx, brw, br); err != nil {
			s.config.ErrorHandler(w, r, s.config.Templates.Funcs(s.buildFuncMap(r, nil)), err)
			return
		}
	})

	s.Handle(pattern, hh, opts...)
}

func (s *Server) buildBrowserMiddlewareStack(h http.Handler, opts ...HandleOpt) http.Handler {
	// Check if we should skip CSRF protection
	skipCSRF := slices.ContainsFunc(opts, func(o HandleOpt) bool {
		_, ok := o.(HandleSkipCSRF)
		return ok
	})

	// Apply secfetch protection
	if skipCSRF {
		// If skipping CSRF, we'll still apply secfetch but allow cross-site requests
		h = secfetch.Protect(h, secfetch.AllowCrossSiteNavigation{}, secfetch.AllowCrossSiteAPI{})
	} else {
		// Standard protection
		h = secfetch.Protect(h)
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

	return h
}

// BrowserHandler creates a new httpHandler from a higher-level abstraction,
// targeted towards responding to browsers.
func (s *Server) BrowserHandler(h BrowserHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			s.config.ErrorHandler(w, r, s.config.Templates.Funcs(s.buildFuncMap(r, nil)), fmt.Errorf("parsing form: %w", err))
			return
		}

		// Create request with server in context
		ctx := WithServer(r.Context(), s)
		r = r.WithContext(ctx)

		br := &Request{
			rw: w,
			r:  r,
		}

		// Create response writer
		rw := NewResponseWriter(w, r, s)

		// Call handler with response writer
		err := h(ctx, rw, br)
		if err != nil {
			s.config.ErrorHandler(w, r, s.config.Templates.Funcs(s.buildFuncMap(r, nil)), err)
			return
		}
	})
}

// VerifyCSRFToken is deprecated and always returns true since we've moved to Sec-Fetch headers
// for CSRF protection. Existing code can continue using this method but it will have no effect.
//
// Deprecated: CSRF protection is now handled by Sec-Fetch headers via the secfetch package.
func (s *Server) VerifyCSRFToken(r *Request, token string) bool {
	// With secfetch, the CSRF verification happens at the middleware level
	// based on the Sec-Fetch-* headers, so no token verification is needed.
	return true
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, staticPrefix) {
		s.staticHandler.ServeHTTP(w, r)
		return
	}
	s.mux.ServeHTTP(w, r)
}

// baseWrappers installs the lowest-level handlers that all requests should
// have, like logging etc.
func (s *Server) baseWrappers(h http.Handler) http.Handler {
	hh := loggingMiddleware(h)
	hh = requestid.Handler(true, hh)
	return hh
}

func (s *Server) csrfFailureHandler(w http.ResponseWriter, r *http.Request) {
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
