package internal

import "net/http"

// UnwrappableResponseWriter is a type that can be unwrapped to get the
// underlying ResponseWriter. It matches stdlib behavior, used to guard our
// implementations.
type UnwrappableResponseWriter interface {
	http.ResponseWriter
	Unwrap() http.ResponseWriter
}

// UnwrapResponseWriterTo walks back the chain of ResponseWriters
// until it finds one that implements the target interface.
// It returns the found ResponseWriter or nil if not found.
func UnwrapResponseWriterTo[T any](rw http.ResponseWriter) (T, bool) {
	currentRW := rw
	for {
		if target, ok := currentRW.(T); ok {
			return target, true
		}

		if unwrapper, ok := currentRW.(interface {
			Unwrap() http.ResponseWriter
		}); ok {
			currentRW = unwrapper.Unwrap()
		} else {
			var zero T
			return zero, false
		}
	}
}
