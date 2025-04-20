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
