package slogctx

import (
	"context"
	"log/slog"
	"sync"
)

// AttributeExtractor is a function that can extract attributes from a context
type AttributeExtractor func(ctx context.Context) []slog.Attr

type namedExtractor struct {
	name      string
	extractor AttributeExtractor
}

var (
	extractors     = make(map[string]namedExtractor)
	extractorsLock sync.RWMutex
)

// RegisterAttributeExtractor registers a new attribute extractor function with the given name.
// If an extractor with the same name already exists, it will be replaced.
// This function is safe to call from init() functions.
func RegisterAttributeExtractor(name string, extractor AttributeExtractor) {
	extractorsLock.Lock()
	defer extractorsLock.Unlock()
	extractors[name] = namedExtractor{
		name:      name,
		extractor: extractor,
	}
}

// DeregisterAttributeExtractor removes an attribute extractor by name.
// Returns true if an extractor was removed, false if no extractor was found with that name.
func DeregisterAttributeExtractor(name string) bool {
	extractorsLock.Lock()
	defer extractorsLock.Unlock()
	_, exists := extractors[name]
	delete(extractors, name)
	return exists
}

// ListExtractors returns a list of all registered extractor names.
func ListExtractors() []string {
	extractorsLock.RLock()
	defer extractorsLock.RUnlock()
	names := make([]string, 0, len(extractors))
	for name := range extractors {
		names = append(names, name)
	}
	return names
}

// getExtractedAttributes returns all attributes from registered extractors
func getExtractedAttributes(ctx context.Context) []slog.Attr {
	extractorsLock.RLock()
	defer extractorsLock.RUnlock()

	var attrs []slog.Attr
	for _, extractor := range extractors {
		if extracted := extractor.extractor(ctx); len(extracted) > 0 {
			attrs = append(attrs, extracted...)
		}
	}
	return attrs
}
