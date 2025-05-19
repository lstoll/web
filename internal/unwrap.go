package internal

import "net/http"

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
