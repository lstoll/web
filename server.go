package web

import (
	"crypto/rand"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"

	"filippo.io/csrf"
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

// NoopHandler can be used to explicitly opt-out of a handler.
func NoopHandler(h http.Handler) http.Handler {
	return h
}

type Config struct {
	BaseURL        *url.URL
	SessionManager *session.Manager
	ErrorHandler   func(w http.ResponseWriter, r *http.Request, code int, err error)
	Static         fs.FS
	CSPOpts        []csp.HandlerOpt
	// ScriptNonce indicates that a nonce should be used for inline scripts.
	// This will update the CSP, and the template func will return a value.
	ScriptNonce bool
	// ScriptNonce indicates that a nonce should be used for inline styles. This
	// will update the CSP, and the template func will return a value.
	StyleNonce bool
	// Mux that handlers will mount on. If nil, a new mux will be created. If an
	// existing mux is passed, a handler for the static content will be added.
	Mux *http.ServeMux
	// AdditionalBrowserMiddleware is a set of middleware that will be added to
	// all browser handlers, after the base middleware.
	AdditionalBrowserMiddleware []func(http.Handler) http.Handler

	/* start new section */
	CSRFHandler func(http.Handler) http.Handler
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

	webMiddleware := []func(http.Handler) http.Handler{}

	csrfHandler := c.CSRFHandler
	if csrfHandler == nil {
		ch := csrf.New()
		csrfHandler = ch.Handler
	}

	webMiddleware = append(webMiddleware, csrfHandler)

	baseMiddleware := []func(http.Handler) http.Handler{
		loggingMiddleware,
		func(h http.Handler) http.Handler {
			// TODO - make requestID be a normal middleware
			return requestid.Handler(true, h)
		},
	}

	svr := &Server{
		config:                   c,
		staticHandler:            sh,
		mux:                      c.Mux,
		baseMiddleware:           baseMiddleware,
		invokeWithBaseMiddleware: buildMiddlewareChain(baseMiddleware),
		invokeWithWebMiddleware:  buildMiddlewareChain(webMiddleware),
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

	baseMiddleware []func(http.Handler) http.Handler
	// invokeWithBaseMiddleware is a pre-built function that applies the base middleware chain
	invokeWithBaseMiddleware func(http.Handler) http.Handler
	// invokeWithWebMiddleware is a pre-built function that applies the web middleware chain
	invokeWithWebMiddleware func(http.Handler) http.Handler
}

func (s *Server) Session() *session.Manager {
	return s.config.SessionManager
}

type HandleBrowserOpts struct {
	SkipCSRF bool
}

// HandleSkipCSRF is an opt that can be passed to a HandleBrowser to skip CSRF
// protection.
//
// Deprecated: CSRF should always be passed via form or header for relevant
// actions. Exceptions should be documented.
type HandleSkipCSRF struct{}

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
	s.mux.Handle(pattern, s.invokeWithBaseMiddleware(handler))
}

func (s *Server) Handle(pattern string, h http.Handler, opts ...HandlerOpt) {
	s.mux.Handle(pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, opt := range opts {
			r = opt(r)
		}
		s.invokeWithWebMiddleware(h).ServeHTTP(w, r)
	}))
}

func (s *Server) HandleFunc(pattern string, h func(w http.ResponseWriter, r *http.Request), opts ...HandlerOpt) {
	s.Handle(pattern, http.HandlerFunc(h), opts...)
}

// func (s *Server) HandleBrowserFunc(pattern string, h BrowserHandlerFunc, opts ...optshandler.HandlerOpt) {
// 	hh := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		// Create request with server in context
// 		ctx := WithServer(r.Context(), s)
// 		r = r.WithContext(ctx)

// 		// Create responsewriter and request
// 		brw := newReseponseWriter(w, r, s)
// 		br := &Request{r: r}

// 		if err := h(ctx, brw, br); err != nil {
// 			s.config.ErrorHandler(w, r, s.config.Templates.Funcs(s.buildFuncMap(r, nil)), err)
// 			return
// 		}
// 	})

// 	s.Handle(pattern, hh)
// }

// func (s *Server) buildBrowserMiddlewareStack(h http.Handler, opts ...optshandler.HandlerOpt) http.Handler {
// 	// Check if we should skip CSRF protection
// 	skipCSRF := slices.ContainsFunc(opts, func(o optshandler.HandlerOpt) bool {
// 		_, ok := o.(HandleSkipCSRF)
// 		return ok
// 	})

// 	// Apply secfetch protection
// 	if skipCSRF {
// 		// If skipping CSRF, we'll still apply secfetch but allow cross-site requests

// 		//h = secfetch.Protect(h, secfetch.AllowCrossSiteNavigation{}, secfetch.AllowCrossSiteAPI{})
// 	} else {
// 		// Standard protection
// 		//h = secfetch.Protect(h)
// 	}

// 	// prevh := h
// 	// h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 	// 	if s.config.BrowserAuthMiddleware != nil {
// 	// 		s.BrowserHandler(s.config.BrowserAuthMiddleware(prevh, opts...)).ServeHTTP(w, r)
// 	// 	} else {
// 	// 		prevh.ServeHTTP(w, r)
// 	// 	}
// 	// })
// 	h = s.config.SessionManager.Wrap(h)
// 	h = s.cspHandler(h)
// 	h = cors.DenyPreflight(h)
// 	h = s.baseWrappers(h)

// 	return h
// }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO - errors etc.
	brw := newReseponseWriter(w, r, s)
	s.mux.ServeHTTP(brw, r)
}

// baseWrappers installs the lowest-level handlers that all requests should
// have, like logging etc.
func (s *Server) baseWrappers(h http.Handler) http.Handler {
	hh := loggingMiddleware(h)
	hh = requestid.Handler(true, hh)
	return hh
}

func buildMiddlewareChain(chain []func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		for i := len(chain) - 1; i >= 0; i-- {
			h = chain[i](h)
		}
		return h
	}
}
