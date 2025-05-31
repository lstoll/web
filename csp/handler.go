package csp

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

type scriptNonceKey struct{}
type styleNonceKey struct{}

// Handler enforces Content-Security-Policy and also reports CSP errors to
// a logger for analysis.
//
// See https://content-security-policy.com/ for documentation on CSP and each
// type.
type Handler struct {
	baseURL    url.URL
	reportsURL url.URL

	reportOnly bool

	defaultSrc     []string
	scriptSrc      []string
	styleSrc       []string
	imgSrc         []string
	connectSrc     []string
	fontSrc        []string
	objectSrc      []string
	mediaSrc       []string
	baseURI        string
	frameAncestors []string
	formAction     []string

	enableScriptNonce bool
	enableStyleNonce  bool
}

type HandlerOpt func(h *Handler)

func ReportOnly(reportOnly bool) HandlerOpt {
	return func(h *Handler) {
		h.reportOnly = reportOnly
	}
}

func DefaultSrc(src ...string) HandlerOpt {
	return func(h *Handler) {
		h.defaultSrc = append(h.defaultSrc, src...)
	}
}

func ScriptSrc(src ...string) HandlerOpt {
	return func(h *Handler) {
		h.scriptSrc = append(h.scriptSrc, src...)
	}
}

func StyleSrc(src ...string) HandlerOpt {
	return func(h *Handler) {
		h.styleSrc = append(h.styleSrc, src...)
	}
}

func ImgSrc(src ...string) HandlerOpt {
	return func(h *Handler) {
		h.imgSrc = append(h.imgSrc, src...)
	}
}

func ConnectSrc(src ...string) HandlerOpt {
	return func(h *Handler) {
		h.connectSrc = append(h.connectSrc, src...)
	}
}

func FontSrc(src ...string) HandlerOpt {
	return func(h *Handler) {
		h.fontSrc = append(h.fontSrc, src...)
	}
}

func ObjectSrc(newSources ...string) HandlerOpt {
	return func(h *Handler) {
		if len(newSources) == 0 {
			return // No change if no new sources are provided
		}

		if slices.Contains(newSources, "'none'") {
			// If 'none' is anywhere in the new sources, the directive becomes 'none'.
			h.objectSrc = []string{"'none'"}
		} else {
			// New sources do not contain 'none'.
			// If the current value is exactly ["'none'"] (our default), replace it.
			if len(h.objectSrc) == 1 && h.objectSrc[0] == "'none'" {
				h.objectSrc = newSources
			} else {
				// Otherwise, append the new (non-'none') sources.
				h.objectSrc = append(h.objectSrc, newSources...)
			}
		}
	}
}

func MediaSrc(src ...string) HandlerOpt {
	return func(h *Handler) {
		h.mediaSrc = append(h.mediaSrc, src...)
	}
}

func BaseURI(src string) HandlerOpt {
	return func(h *Handler) {
		h.baseURI = src
	}
}

func FrameAncestors(src ...string) HandlerOpt {
	return func(h *Handler) {
		h.frameAncestors = append(h.frameAncestors, src...)
	}
}

func FormAction(src ...string) HandlerOpt {
	return func(h *Handler) {
		h.formAction = append(h.formAction, src...)
	}
}

// WithScriptNonce enables dynamic nonce generation for script-src.
// A unique nonce will be generated per request and added to the script-src directive.
// Use GetScriptNonce(ctx) to retrieve it for use in templates.
func WithScriptNonce() HandlerOpt {
	return func(h *Handler) {
		h.enableScriptNonce = true
	}
}

// WithStyleNonce enables dynamic nonce generation for style-src.
// A unique nonce will be generated per request and added to the style-src directive.
// Use GetStyleNonce(ctx) to retrieve it for use in templates.
func WithStyleNonce() HandlerOpt {
	return func(h *Handler) {
		h.enableStyleNonce = true
	}
}

func NewHandler(baseURL url.URL, opts ...HandlerOpt) *Handler {
	h := &Handler{
		baseURL: baseURL,
	}

	reportsURL := baseURL // copy
	reportsURL.Path += "/_/csp-reports"
	h.reportsURL = reportsURL

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// DefaultOpts provides a slice of HandlerOpt for a recommended secure default CSP.
// It enables script and style nonces by default.
// Users can append their own HandlerOpt to this slice to customize the policy.
// Example:
//
//	myCustomOpts := []csp.HandlerOpt{ csp.ImgSrc("https://mycdn.com") }
//	allOpts := append(csp.DefaultOpts, myCustomOpts...)
//	handler := csp.NewHandler(baseURL, allOpts...)
var DefaultOpts = []HandlerOpt{
	DefaultSrc("'self'"),
	ScriptSrc("'self'"),
	StyleSrc("'self'"),
	WithScriptNonce(),
	WithStyleNonce(),
	ImgSrc("'self'", "data:"),
	FontSrc("'self'"),
	ObjectSrc("'none'"),
	BaseURI("'self'"),
	FormAction("'self'"),
	FrameAncestors("'self'"),
	ConnectSrc("'self'"),
}

// Wrap wraps an existing http.Handler with the configured content security
// policy. It also intercepts POST requests to /_/csp-reports and logs them as
// CSP violations. Nonces are generated here if enabled.
func (h *Handler) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if h.enableScriptNonce {
			ctx = context.WithValue(ctx, scriptNonceKey{}, rand.Text())
		}

		if h.enableStyleNonce {
			ctx = context.WithValue(ctx, styleNonceKey{}, rand.Text())
		}

		r = r.WithContext(ctx)
		h.addCSPHeaders(w, r)

		if r.Method == http.MethodPost && r.URL.Path == h.reportsURL.Path {
			violation, err := io.ReadAll(r.Body)
			if err != nil {
				slog.ErrorContext(r.Context(), "reading CSP violation body", "err", err) // Use original context for error reporting
				http.Error(w, "Failed to read CSP report", http.StatusInternalServerError)
				return
			}
			slog.InfoContext(r.Context(), "CSP violation", slog.String("violation", string(violation))) // Use original context
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *Handler) addCSPHeaders(w http.ResponseWriter, r *http.Request) {
	var elements []string

	// Helper to build a directive string
	buildDirective := func(name string, values []string) string {
		if len(values) == 0 {
			return ""
		}
		return fmt.Sprintf("%s %s", name, strings.Join(values, " "))
	}

	if d := buildDirective("default-src", h.defaultSrc); d != "" {
		elements = append(elements, d)
	}

	scriptSrcValues := make([]string, len(h.scriptSrc))
	copy(scriptSrcValues, h.scriptSrc)
	if h.enableScriptNonce {
		nonce, ok := GetScriptNonce(r.Context())
		if !ok {
			// If WithScriptNonce() was called, a nonce should always be in the context.
			// Panicking here indicates an unexpected internal state.
			panic("CSP: script nonce enabled but not found in context")
		}
		scriptSrcValues = append(scriptSrcValues, fmt.Sprintf("'nonce-%s'", nonce))
	}
	if d := buildDirective("script-src", scriptSrcValues); d != "" {
		elements = append(elements, d)
	}

	styleSrcValues := make([]string, len(h.styleSrc))
	copy(styleSrcValues, h.styleSrc)
	if h.enableStyleNonce {
		nonce, ok := GetStyleNonce(r.Context())
		if !ok {
			// If WithStyleNonce() was called, a nonce should always be in the context.
			panic("CSP: style nonce enabled but not found in context")
		}
		styleSrcValues = append(styleSrcValues, fmt.Sprintf("'nonce-%s'", nonce))
	}
	if d := buildDirective("style-src", styleSrcValues); d != "" {
		elements = append(elements, d)
	}

	if d := buildDirective("img-src", h.imgSrc); d != "" {
		elements = append(elements, d)
	}
	if d := buildDirective("connect-src", h.connectSrc); d != "" {
		elements = append(elements, d)
	}
	if d := buildDirective("font-src", h.fontSrc); d != "" {
		elements = append(elements, d)
	}
	if d := buildDirective("object-src", h.objectSrc); d != "" {
		elements = append(elements, d)
	}
	if d := buildDirective("media-src", h.mediaSrc); d != "" {
		elements = append(elements, d)
	}
	if h.baseURI != "" {
		elements = append(elements, fmt.Sprintf("base-uri %s", h.baseURI))
	}
	if d := buildDirective("frame-ancestors", h.frameAncestors); d != "" {
		elements = append(elements, d)
	}
	if d := buildDirective("form-action", h.formAction); d != "" {
		elements = append(elements, d)
	}

	elements = append(elements, fmt.Sprintf("report-uri %s", h.reportsURL.String()))

	headerName := "Content-Security-Policy"
	if h.reportOnly {
		headerName = "Content-Security-Policy-Report-Only"
	}
	if len(elements) > 0 {
		w.Header().Set(headerName, strings.Join(elements, "; "))
	}
}

// GetScriptNonce retrieves the script nonce from the context, if available.
func GetScriptNonce(ctx context.Context) (string, bool) {
	nonce, ok := ctx.Value(scriptNonceKey{}).(string)
	return nonce, ok
}

// GetStyleNonce retrieves the style nonce from the context, if available.
func GetStyleNonce(ctx context.Context) (string, bool) {
	nonce, ok := ctx.Value(styleNonceKey{}).(string)
	return nonce, ok
}
