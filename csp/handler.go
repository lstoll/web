package csp

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// Handler enforces Content-Security-Policy and also reports CSP errors to
// a logger for analysis.
//
// See https://content-security-policy.com/ for documentation on CSP and each
// type.
type Handler struct {
	baseURL    url.URL
	reportsURL url.URL

	reportOnly bool

	defaultSrc     string
	scriptSrc      string
	styleSrc       string
	imgSrc         string
	connectSrc     string
	fontSrc        string
	objectSrc      string
	mediaSrc       string
	baseURI        string
	frameAncestors string
	formAction     string
}

type HandlerOpt func(h *Handler)

func ReportOnly(reportOnly bool) HandlerOpt {
	return func(h *Handler) {
		h.reportOnly = reportOnly
	}
}

func DefaultSrc(src string) HandlerOpt {
	return func(h *Handler) {
		h.defaultSrc = src
	}
}

func ScriptSrc(src string) HandlerOpt {
	return func(h *Handler) {
		h.scriptSrc = src
	}
}

func StyleSrc(src string) HandlerOpt {
	return func(h *Handler) {
		h.styleSrc = src
	}
}

func ImgSrc(src string) HandlerOpt {
	return func(h *Handler) {
		h.imgSrc = src
	}
}

func ConnectSrc(src string) HandlerOpt {
	return func(h *Handler) {
		h.connectSrc = src
	}
}

func FontSrc(src string) HandlerOpt {
	return func(h *Handler) {
		h.fontSrc = src
	}
}

func ObjectSrc(src string) HandlerOpt {
	return func(h *Handler) {
		h.objectSrc = src
	}
}

func MediaSrc(src string) HandlerOpt {
	return func(h *Handler) {
		h.mediaSrc = src
	}
}

func BaseURI(src string) HandlerOpt {
	return func(h *Handler) {
		h.baseURI = src
	}
}

func FrameAncestors(src string) HandlerOpt {
	return func(h *Handler) {
		h.frameAncestors = src
	}
}

func FormAction(src string) HandlerOpt {
	return func(h *Handler) {
		h.formAction = src
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

// Wrap wraps an existing http.Handler with the configured content security
// policy. It also intercepts POST requests to /_/csp-reports and logs them as
// CSP violations.
func (h *Handler) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.addCSPHeaders(w)

		if r.Method != http.MethodPost || r.URL.Path != h.reportsURL.Path {
			next.ServeHTTP(w, r)
			return
		}

		violation, err := io.ReadAll(r.Body)
		if err != nil {
			slog.ErrorContext(r.Context(), "reading body", "err", err)
			return
		}

		slog.InfoContext(r.Context(), "CSP violation", slog.String("violation", string(violation)))
	})
}

func (h *Handler) addCSPHeaders(w http.ResponseWriter) {
	var elements []string

	if h.defaultSrc != "" {
		elements = append(elements, fmt.Sprintf("default-src %s", h.defaultSrc))
	}
	if h.scriptSrc != "" {
		elements = append(elements, fmt.Sprintf("script-src %s", h.scriptSrc))
	}
	if h.styleSrc != "" {
		elements = append(elements, fmt.Sprintf("style-src %s", h.styleSrc))
	}
	if h.imgSrc != "" {
		elements = append(elements, fmt.Sprintf("img-src %s", h.imgSrc))
	}
	if h.connectSrc != "" {
		elements = append(elements, fmt.Sprintf("connect-src %s", h.connectSrc))
	}
	if h.fontSrc != "" {
		elements = append(elements, fmt.Sprintf("font-src %s", h.fontSrc))
	}
	if h.objectSrc != "" {
		elements = append(elements, fmt.Sprintf("object-src %s", h.objectSrc))
	}
	if h.mediaSrc != "" {
		elements = append(elements, fmt.Sprintf("media-src %s", h.mediaSrc))
	}
	if h.baseURI != "" {
		elements = append(elements, fmt.Sprintf("base-uri %s", h.baseURI))
	}
	if h.frameAncestors != "" {
		elements = append(elements, fmt.Sprintf("frame-ancestors %s", h.frameAncestors))
	}
	if h.formAction != "" {
		elements = append(elements, fmt.Sprintf("form-action %s", h.formAction))
	}

	elements = append(elements, fmt.Sprintf("report-uri %s", h.reportsURL.String()))

	headerName := "Content-Security-Policy"
	if h.reportOnly {
		headerName = "Content-Security-Policy-Report-Only"
	}
	w.Header().Set(headerName, strings.Join(elements, "; "))
}
