package proxyhdrs

import (
	"context"
	"net"
	"net/http"
	"strings"
)

const (
	ForwardedIPHeaderXFF         = "X-Forwarded-For"
	ForwardedIPHeaderXRealIP     = "X-Real-IP"
	ForwardedIPHeaderFlyClientIP = "Fly-Client-IP"
)

type contextKeyOriginalRequest struct{}

// OriginalRequestFromContext returns the original request from the context,
// before the header values were used.
func OriginalRequestFromContext(ctx context.Context) (*http.Request, bool) {
	originalRequest, ok := ctx.Value(contextKeyOriginalRequest{}).(*http.Request)
	return originalRequest, ok
}

// ForwardedIPHeaderFormat specifies how the adress should be extracted from the
// ForwardedIPHeader
type ForwardedIPHeaderFormat uint32

const (
	// ForwardedIPHeaderFormatExact means the address is the exact address in the header
	ForwardedIPHeaderFormatExact ForwardedIPHeaderFormat = iota
	// ForwardedIPHeaderFormatFirst means the address is the first address in the header,
	// comma separated.
	ForwardedIPHeaderFormatFirst
	// ForwardedIPHeaderFormatLast means the address is the last address in the
	// header, comma separated.
	ForwardedIPHeaderFormatLast
)

// RemoteIP is used as a middleware to handle re-writing request's IP addresses.
// Used when the app is served behind a proxy that terminates requests and
// re-writes what is sent to the backend.
type RemoteIP struct {
	// ForwardedIPHeader is the header that contains the original IP address.
	// Required.
	ForwardedIPHeader string
	// ForwardedIPHeaderFormat specifies how the address should be extracted
	// from the ForwardedIPHeader.
	ForwardedIPHeaderFormat ForwardedIPHeaderFormat
}

// Handle wraps the handler, re-writing the request's IP address based on the
// ForwardedIPHeader. The original request can be retrieved with
// OriginalRequestFromContext
func (h *RemoteIP) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), contextKeyOriginalRequest{}, r)
		r = r.Clone(ctx)

		if h.ForwardedIPHeader != "" {
			hdr := r.Header.Get(h.ForwardedIPHeader)
			if hdr != "" {
				var ip net.IP

				switch h.ForwardedIPHeaderFormat {
				case ForwardedIPHeaderFormatExact:
					ip = net.ParseIP(hdr)
				case ForwardedIPHeaderFormatFirst:
					// Split by comma and take the first non-empty, trimmed value
					parts := strings.Split(hdr, ",")
					for _, part := range parts {
						trimmed := strings.TrimSpace(part)
						if trimmed != "" {
							ip = net.ParseIP(trimmed)
							break
						}
					}
				case ForwardedIPHeaderFormatLast:
					// Split by comma and take the last non-empty, trimmed value
					parts := strings.Split(hdr, ",")
					for i := len(parts) - 1; i >= 0; i-- {
						trimmed := strings.TrimSpace(parts[i])
						if trimmed != "" {
							ip = net.ParseIP(trimmed)
							break
						}
					}
				}

				if ip != nil {
					r.RemoteAddr = ip.String()
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}
