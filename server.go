package web

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/url"

	"github.com/lstoll/web/csp"
	"github.com/lstoll/web/csrf"
	"github.com/lstoll/web/httperror"
	"github.com/lstoll/web/requestid"
	"github.com/lstoll/web/requestlog"
	"github.com/lstoll/web/session"
	"github.com/lstoll/web/static"
)

const staticPrefix = "/static/"

var DefaultCSPOpts = []csp.HandlerOpt{
	csp.DefaultSrc(`'none'`),
	csp.WithScriptNonce(),
	csp.WithStyleNonce(),
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
	ErrorHandler   func(w http.ResponseWriter, r *http.Request, err error)
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
		c.ErrorHandler = httperror.DefaultErrorHandler
	}

	sh, err := static.NewFileHandler(c.Static, staticPrefix)
	if err != nil {
		return nil, fmt.Errorf("creating static handler: %w", err)
	}

	webMiddleware := []func(http.Handler) http.Handler{}

	csrfHandler := c.CSRFHandler
	if csrfHandler == nil {
		ch := http.NewCrossOriginProtection()
		csrfHandler = csrf.NewHandler(ch).Handler
	}

	webMiddleware = append(webMiddleware, csrfHandler)

	// set the static handler in the context, so we can use it to build paths in
	// templates.
	webMiddleware = append(webMiddleware, func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(contextWithStaticHandler(r.Context(), sh))
			h.ServeHTTP(w, r)
		})
	})

	loghandler := &requestlog.RequestLogger{
		// TODO - pass in something?
	}

	baseMiddleware := []func(http.Handler) http.Handler{
		func(h http.Handler) http.Handler {
			// TODO - make requestID be a normal middleware
			return requestid.Handler(true, h)
		},
		loghandler.Handler,
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
	staticHandler *static.FileHandler

	baseMiddleware []func(http.Handler) http.Handler
	// invokeWithBaseMiddleware is a pre-built function that applies the base middleware chain
	invokeWithBaseMiddleware func(http.Handler) http.Handler
	// invokeWithWebMiddleware is a pre-built function that applies the web middleware chain
	invokeWithWebMiddleware func(http.Handler) http.Handler
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

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO - errors etc.
	brw := newResponseWriter(w, r, s)
	s.mux.ServeHTTP(brw, r)
}

func buildMiddlewareChain(chain []func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		for i := len(chain) - 1; i >= 0; i-- {
			h = chain[i](h)
		}
		return h
	}
}
