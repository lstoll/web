package requestid

import (
	"fmt"
	"net/http"
)

var _ http.RoundTripper = (*Transport)(nil)

// Transport is a http.RoundTripper, that will set the X-Request-ID header on outgoing
// calls if a request ID exists in the request's context
type Transport struct {

	// Base is the base RoundTripper used to make HTTP requests. If nil,
	// http.DefaultTransport is used.
	Base http.RoundTripper
}

// RoundTrip adds the request ID header to the outgoing request, as needed.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqBodyClosed := false
	if req.Body != nil {
		defer func() {
			if !reqBodyClosed {
				_ = req.Body.Close()
			}
		}()
	}

	req2 := req.Clone(req.Context()) // per RoundTripper contract

	requestID, ok := FromContext(req2.Context())
	if ok {
		if req.Header.Get(RequestIDHeader) != "" {
			return nil, fmt.Errorf("requestid: provided request should not have a Request ID header already")
		}

		req2.Header.Set(RequestIDHeader, requestID)
	}

	// req.Body is assumed to be closed by the base RoundTripper.
	reqBodyClosed = true
	return t.base().RoundTrip(req2)
}

func (t *Transport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}
