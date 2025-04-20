package requestid

import "net/http"

// HTTPClientWithRequestID will update the passed *http.Client to add a request
// ID to all outgoing requests, when the request's context contains one.
func HTTPClientWithRequestID(client *http.Client) {
	base := client.Transport
	client.Transport = &Transport{
		Base: base,
	}
}
