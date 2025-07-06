package proxyhdrs

import "net/http"

type ForceTLS struct {
	ForwardedProtoHeader string
	HTTPMux              *http.ServeMux
}

func (h *ForceTLS) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var isHTTPS bool
		if r.TLS != nil {
			isHTTPS = true
		} else if h.ForwardedProtoHeader != "" {
			hdr := r.Header.Get(h.ForwardedProtoHeader)
			if hdr == "https" {
				isHTTPS = true
			}
		}
		if isHTTPS {
			next.ServeHTTP(w, r)
			return
		}

		// do a check if we have a plain-text handler, use it if so.
		if h.HTTPMux != nil {
			h, p := h.HTTPMux.Handler(r)
			if p != "" {
				h.ServeHTTP(w, r)
				return
			}
		}

		// otherwise, redirect to HTTPS
		r.URL.Fragment = ""
		redirectURL := "https://" + r.Host + r.URL.Path
		if r.URL.RawQuery != "" {
			redirectURL += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, redirectURL, http.StatusPermanentRedirect)
	})
}
