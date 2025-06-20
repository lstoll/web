package requestid

import (
	"context"
	"crypto/rand"
	"fmt"
)

type requestIDCtxKey struct{}

// ContextWithRequestID adds the specified request ID to the context
func ContextWithRequestID(parent context.Context, requestID string) context.Context {
	return context.WithValue(parent, requestIDCtxKey{}, requestID)
}

// ContextWithNewRequestID sets a new request ID on the context, returning the
// ID.
func ContextWithNewRequestID(parent context.Context) (_ context.Context, requestID string) {
	rid := newRequestID()
	return context.WithValue(parent, requestIDCtxKey{}, rid), rid
}

// FromContext returns the request ID from the context. If there is no
// request ID in the context, ok will be false.
func FromContext(ctx context.Context) (_ string, ok bool) {
	v, ok := ctx.Value(requestIDCtxKey{}).(string)
	return v, ok
}

// newRequestID generates a new uuidv4
func newRequestID() string {
	var uuid [16]byte

	// Fill the entire UUID with cryptographically secure random bytes.
	if _, err := rand.Read(uuid[:]); err != nil {
		panic(fmt.Sprintf("requestid: reading random bytes: %v", err))
	}

	// Set the version to 4.
	// The 7th byte's most significant 4 bits are set to 0100.
	// uuid[6] = (uuid[6] & 0x0F) | 0x40
	// Clear the first 4 bits and then set the 4th bit.
	uuid[6] = (uuid[6] & 0b00001111) | 0b01000000

	// Set the variant to RFC 4122 (10xx).
	// The 9th byte's most significant 2 bits are set to 10.
	// uuid[8] = (uuid[8] & 0x3F) | 0x80
	// Clear the first 2 bits and then set the 2nd bit.
	uuid[8] = (uuid[8] & 0b00111111) | 0b10000000

	// Format the UUID into the standard 8-4-4-4-12 string representation.
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}
