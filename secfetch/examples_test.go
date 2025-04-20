package secfetch

import (
	"net/http"
)

// This file contains examples for using the secfetch package

func ExampleProtect() {
	// Create a handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Hello, World!"))
	})

	// Wrap with secfetch protection
	protected := Protect(handler)

	// Use the protected handler
	http.Handle("/", protected)
}

func ExampleProtect_withOptions() {
	// Create a handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("API response"))
	})

	// Wrap with secfetch protection and custom options
	protected := Protect(
		handler,
		AllowCrossSiteAPI{},
		WithAllowedModes("navigate", "cors"),
		WithAllowedDests("document", "empty"),
	)

	// Use the protected handler
	http.Handle("/api", protected)
}

func ExampleWebProtect() {
	// In a real application, this would be using the web package
	// Here we're just showing the usage pattern

	// Create a handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Protected with HandleOpt"))
	})

	// Mock server.HandleBrowser with WebProtect
	protected := WebProtect(handler)

	// Use the protected handler
	http.Handle("/with-handleopt", protected)
}

func ExampleWebProtect_withOptions() {
	// In a real application, this would be using the web package
	// Here we're just showing the usage pattern

	// Create a handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Protected with HandleOpt and options"))
	})

	// Simulate passing options via HandleOpt
	mockWebOpts := []interface{}{
		AllowNav{},
		AllowAPI{},
		SecFetchOpt{
			Options: []Option{
				WithAllowedModes("navigate", "cors"),
			},
		},
	}

	// Apply protection with options
	protected := WebProtect(handler, mockWebOpts...)

	// Use the protected handler
	http.Handle("/with-handleopt-and-options", protected)
}
