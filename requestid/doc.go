// Package requestid allows the generation and propagation of request ID's via
// context, and HTTP calls.
package requestid

// RequestIDHeader is the HTTP header name that we pass the request ID in.
const RequestIDHeader = "X-Request-ID"
