package web

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"strings"

	"github.com/lstoll/web/csp"
	"github.com/lstoll/web/csrf"
	"github.com/lstoll/web/httperror"
	"github.com/lstoll/web/middleware"
	"github.com/lstoll/web/requestid"
	"github.com/lstoll/web/requestlog"
	"github.com/lstoll/web/session"
	"github.com/lstoll/web/static"
)

const staticPrefix = "/static/"

const (
	MiddlewareCSPName        = "csp"
	MiddlewareCSRFName       = "csrf"
	MiddlewareRequestIDName  = "requestid"
	MiddlewareRequestLogName = "requestlog"
	MiddlewareSessionName    = "session"
	MiddlewareErrorName      = "error"
	MiddlewareStaticName     = "static"
)

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
	if c.ErrorHandler == nil {
		c.ErrorHandler = httperror.DefaultErrorHandler
	}

	sh, err := static.NewFileHandler(c.Static, staticPrefix)
	if err != nil {
		return nil, fmt.Errorf("creating static handler: %w", err)
	}

	csrfHandler := c.CSRFHandler
	if csrfHandler == nil {
		csrfHandler = csrf.New().Handler
	}

	cspHandler := csp.NewHandler(*c.BaseURL, c.CSPOpts...)

	loghandler := &requestlog.RequestLogger{
		// TODO - pass in something?
	}

	svr := &Server{
		config:            c,
		staticHandler:     sh,
		BrowserMux:        http.NewServeMux(),
		RawMux:            http.NewServeMux(),
		BrowserMiddleware: &middleware.Chain{},
		BaseMiddleware:    &middleware.Chain{},
	}

	svr.BaseMiddleware.Append(MiddlewareRequestIDName, func(h http.Handler) http.Handler {
		// TODO - make requestID be a normal middleware
		return requestid.Handler(true, h)
	})
	svr.BaseMiddleware.Append(MiddlewareRequestLogName, loghandler.Handler)
	svr.BaseMiddleware.Append(MiddlewareErrorName, (&httperror.Handler{}).Handle)

	svr.BrowserMiddleware.Append(MiddlewareStaticName, func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// set the static handler in the context, so we can use it to build paths in
			// templates.
			r = r.WithContext(contextWithStaticHandler(r.Context(), sh))
			h.ServeHTTP(w, r)
		})
	})
	svr.BrowserMiddleware.Append(MiddlewareCSPName, cspHandler.Wrap)
	svr.BrowserMiddleware.Append(MiddlewareCSRFName, csrfHandler)
	if c.SessionManager != nil {
		svr.BrowserMiddleware.Append(MiddlewareSessionName, c.SessionManager.Wrap)
	}

	svr.RawMux.Handle("/static/", svr.staticHandler)

	return svr, nil
}

type Server struct {
	BrowserMux        *http.ServeMux
	BrowserMiddleware *middleware.Chain

	RawMux *http.ServeMux

	BaseMiddleware *middleware.Chain

	HTTPServer *http.Server

	config        *Config
	staticHandler *static.FileHandler
}

func (s *Server) HandleRaw(pattern string, handler http.Handler) {
	s.RawMux.Handle(pattern, handler)
}

func (s *Server) Handle(pattern string, h http.Handler, opts ...HandlerOpt) {
	s.BrowserMux.Handle(pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, opt := range opts {
			r = opt(r)
		}
		brw := newResponseWriter(w, r, s)
		h.ServeHTTP(brw, r)
	}))
}

func (s *Server) HandleFunc(pattern string, h func(w http.ResponseWriter, r *http.Request), opts ...HandlerOpt) {
	s.Handle(pattern, http.HandlerFunc(h), opts...)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bh, bp := s.BrowserMux.Handler(r)
	rh, rp := s.RawMux.Handler(r)

	switch {
	case bp != "" && rp == "":
		// browser path only
		s.BaseMiddleware.Handler(s.BrowserMiddleware.Handler(bh)).ServeHTTP(w, r)
		return
	case bp == "" && rp != "":
		// raw path only
		s.BaseMiddleware.Handler(rh).ServeHTTP(w, r)
		return
	case bp != "" && rp != "":
		switch compareSpecificity(bp, rp) {
		case 1:
			s.BaseMiddleware.Handler(s.BrowserMiddleware.Handler(bh)).ServeHTTP(w, r)
			return
		case -1:
			s.BaseMiddleware.Handler(rh).ServeHTTP(w, r)
			return
		default:
			// TODO - error handler for this too?
			s.BaseMiddleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "Duplicate route", http.StatusInternalServerError)
			})).ServeHTTP(w, r)
			return
		}
	default:
		// not found
		// TODO - call the error handler directly?
		s.BaseMiddleware.Handler(http.NotFoundHandler()).ServeHTTP(w, r)
		return
	}
}

// compareSpecificity determines the relative specificity of two patterns.
// It returns:
//
//	+1 if pattern1 is more specific than pattern2
//	-1 if pattern2 is more specific than pattern1
//	 0 if they have equal specificity
func compareSpecificity(pattern1, pattern2 string) int {
	if pattern1 == pattern2 {
		return 0
	}

	// Rule 1: Host specificity
	host1 := hasHost(pattern1)
	host2 := hasHost(pattern2)
	if host1 && !host2 {
		return 1
	}
	if !host1 && host2 {
		return -1
	}

	method1, path1 := splitPattern(pattern1)
	method2, path2 := splitPattern(pattern2)

	// Rule 2: Method specificity
	if method1 != "" && method2 == "" {
		return 1
	}
	if method1 == "" && method2 != "" {
		return -1
	}

	// Rule 3: Path segment count
	segments1 := countSegments(path1)
	segments2 := countSegments(path2)
	if segments1 > segments2 {
		return 1
	}
	if segments1 < segments2 {
		return -1
	}

	// Rule 4: Wildcard count (if segment counts are equal)
	wildcards1 := strings.Count(path1, "{")
	wildcards2 := strings.Count(path2, "{")
	if wildcards1 < wildcards2 {
		return 1
	}
	if wildcards1 > wildcards2 {
		return -1
	}

	return 0
}

// splitPattern is the same helper function as before.
func splitPattern(pattern string) (method, path string) {
	if parts := strings.SplitN(pattern, " ", 2); len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", pattern
}

// countSegments counts the number of non-empty parts in a URL path.
// e.g., "/api/v1" -> 2, "/users/" -> 1, "/" -> 0
func countSegments(path string) int {
	// Trim leading/trailing slashes to handle cases like "/" or "/users/" consistently
	trimmedPath := strings.Trim(path, "/")
	if trimmedPath == "" {
		return 0
	}
	return strings.Count(trimmedPath, "/") + 1
}

// hasHost correctly determines if a pattern includes a host.
func hasHost(pattern string) bool {
	// We only care about the part after a potential method.
	_, pathPart := splitPattern(pattern)

	// A pattern has a host if it contains a slash but doesn't start with one.
	// e.g., "example.com/path" contains "/" but doesn't start with "/" -> has host
	// e.g., "/path" contains "/" and starts with "/" -> no host
	return strings.Contains(pathPart, "/") && !strings.HasPrefix(pathPart, "/")
}
