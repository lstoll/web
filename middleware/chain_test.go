package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestChain_Append(t *testing.T) {
	tests := []struct {
		name     string
		handlers []struct {
			name string
			fn   func(next http.Handler) http.Handler
		}
		wantNames []string
	}{
		{
			name: "append single handler",
			handlers: []struct {
				name string
				fn   func(next http.Handler) http.Handler
			}{
				{"handler1", nil},
			},
			wantNames: []string{"handler1"},
		},
		{
			name: "append multiple handlers",
			handlers: []struct {
				name string
				fn   func(next http.Handler) http.Handler
			}{
				{"handler1", nil},
				{"handler2", nil},
				{"handler3", nil},
			},
			wantNames: []string{"handler1", "handler2", "handler3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := &Chain{}

			for _, h := range tt.handlers {
				chain.Append(h.name, h.fn)
			}

			got := chain.List()
			if diff := cmp.Diff(tt.wantNames, got); diff != "" {
				t.Errorf("Append() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestChain_Prepend(t *testing.T) {
	tests := []struct {
		name      string
		initial   []string
		prepend   string
		wantNames []string
	}{
		{
			name:      "prepend to empty chain",
			initial:   []string{},
			prepend:   "handler0",
			wantNames: []string{"handler0"},
		},
		{
			name:      "prepend to existing chain",
			initial:   []string{"handler1", "handler2"},
			prepend:   "handler0",
			wantNames: []string{"handler0", "handler1", "handler2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := &Chain{}

			// Add initial handlers
			for _, name := range tt.initial {
				chain.Append(name, nil)
			}

			// Prepend new handler
			chain.Prepend(tt.prepend, nil)

			got := chain.List()
			if diff := cmp.Diff(tt.wantNames, got); diff != "" {
				t.Errorf("Prepend() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestChain_InsertBefore(t *testing.T) {
	tests := []struct {
		name        string
		initial     []string
		insertName  string
		beforeName  string
		wantNames   []string
		wantErr     bool
		expectedErr string
	}{
		{
			name:       "insert before existing handler",
			initial:    []string{"handler1", "handler2", "handler3"},
			insertName: "inserted",
			beforeName: "handler2",
			wantNames:  []string{"handler1", "inserted", "handler2", "handler3"},
			wantErr:    false,
		},
		{
			name:       "insert before first handler",
			initial:    []string{"handler1", "handler2"},
			insertName: "inserted",
			beforeName: "handler1",
			wantNames:  []string{"inserted", "handler1", "handler2"},
			wantErr:    false,
		},
		{
			name:        "insert before non-existent handler",
			initial:     []string{"handler1", "handler2"},
			insertName:  "inserted",
			beforeName:  "nonexistent",
			wantNames:   []string{"handler1", "handler2"},
			wantErr:     true,
			expectedErr: "handler nonexistent not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := &Chain{}

			// Add initial handlers
			for _, name := range tt.initial {
				chain.Append(name, nil)
			}

			// Insert before target
			err := chain.InsertBefore(tt.beforeName, func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(tt.insertName))
					next.ServeHTTP(w, r)
				})
			})

			// Check error
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if err.Error() != tt.expectedErr {
					t.Errorf("Expected error '%s', got '%s'", tt.expectedErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			got := chain.List()
			if diff := cmp.Diff(tt.wantNames, got); diff != "" {
				t.Errorf("InsertBefore() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestChain_InsertAfter(t *testing.T) {
	tests := []struct {
		name        string
		initial     []string
		insertName  string
		afterName   string
		wantNames   []string
		wantErr     bool
		expectedErr string
	}{
		{
			name:       "insert after existing handler",
			initial:    []string{"handler1", "handler2", "handler3"},
			insertName: "inserted",
			afterName:  "handler2",
			wantNames:  []string{"handler1", "handler2", "inserted", "handler3"},
			wantErr:    false,
		},
		{
			name:       "insert after last handler",
			initial:    []string{"handler1", "handler2"},
			insertName: "inserted",
			afterName:  "handler2",
			wantNames:  []string{"handler1", "handler2", "inserted"},
			wantErr:    false,
		},
		{
			name:        "insert after non-existent handler",
			initial:     []string{"handler1", "handler2"},
			insertName:  "inserted",
			afterName:   "nonexistent",
			wantNames:   []string{"handler1", "handler2"},
			wantErr:     true,
			expectedErr: "handler nonexistent not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := &Chain{}

			// Add initial handlers
			for _, name := range tt.initial {
				chain.Append(name, nil)
			}

			// Insert after target
			err := chain.InsertAfter(tt.afterName, func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(tt.insertName))
					next.ServeHTTP(w, r)
				})
			})

			// Check error
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if err.Error() != tt.expectedErr {
					t.Errorf("Expected error '%s', got '%s'", tt.expectedErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			got := chain.List()
			if diff := cmp.Diff(tt.wantNames, got); diff != "" {
				t.Errorf("InsertAfter() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestChain_Remove(t *testing.T) {
	tests := []struct {
		name        string
		initial     []string
		removeName  string
		wantNames   []string
		wantErr     bool
		expectedErr string
	}{
		{
			name:       "remove middle handler",
			initial:    []string{"handler1", "handler2", "handler3"},
			removeName: "handler2",
			wantNames:  []string{"handler1", "handler3"},
			wantErr:    false,
		},
		{
			name:       "remove first handler",
			initial:    []string{"handler1", "handler2", "handler3"},
			removeName: "handler1",
			wantNames:  []string{"handler2", "handler3"},
			wantErr:    false,
		},
		{
			name:       "remove last handler",
			initial:    []string{"handler1", "handler2", "handler3"},
			removeName: "handler3",
			wantNames:  []string{"handler1", "handler2"},
			wantErr:    false,
		},
		{
			name:        "remove non-existent handler",
			initial:     []string{"handler1", "handler2"},
			removeName:  "nonexistent",
			wantNames:   []string{"handler1", "handler2"},
			wantErr:     true,
			expectedErr: "handler nonexistent not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := &Chain{}

			// Add initial handlers
			for _, name := range tt.initial {
				chain.Append(name, nil)
			}

			// Remove target
			err := chain.Remove(tt.removeName)

			// Check error
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if err.Error() != tt.expectedErr {
					t.Errorf("Expected error '%s', got '%s'", tt.expectedErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			got := chain.List()
			if diff := cmp.Diff(tt.wantNames, got); diff != "" {
				t.Errorf("Remove() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestChain_Replace(t *testing.T) {
	tests := []struct {
		name        string
		initial     []string
		replaceName string
		wantNames   []string
		wantErr     bool
		expectedErr string
	}{
		{
			name:        "replace existing handler",
			initial:     []string{"handler1", "handler2", "handler3"},
			replaceName: "handler2",
			wantNames:   []string{"handler1", "handler2", "handler3"},
			wantErr:     false,
		},
		{
			name:        "replace non-existent handler",
			initial:     []string{"handler1", "handler2"},
			replaceName: "nonexistent",
			wantNames:   []string{"handler1", "handler2"},
			wantErr:     true,
			expectedErr: "handler nonexistent not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := &Chain{}

			// Add initial handlers
			for _, name := range tt.initial {
				chain.Append(name, nil)
			}

			// Replace target
			err := chain.Replace(tt.replaceName, func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("replaced"))
					next.ServeHTTP(w, r)
				})
			})

			// Check error
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if err.Error() != tt.expectedErr {
					t.Errorf("Expected error '%s', got '%s'", tt.expectedErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			got := chain.List()
			if diff := cmp.Diff(tt.wantNames, got); diff != "" {
				t.Errorf("Replace() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestChain_List(t *testing.T) {
	tests := []struct {
		name      string
		handlers  []string
		wantNames []string
	}{
		{
			name:      "empty chain",
			handlers:  []string{},
			wantNames: []string{},
		},
		{
			name:      "single handler",
			handlers:  []string{"handler1"},
			wantNames: []string{"handler1"},
		},
		{
			name:      "multiple handlers",
			handlers:  []string{"handler1", "handler2", "handler3"},
			wantNames: []string{"handler1", "handler2", "handler3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := &Chain{}

			// Add handlers
			for _, name := range tt.handlers {
				chain.Append(name, nil)
			}

			got := chain.List()
			if diff := cmp.Diff(tt.wantNames, got); diff != "" {
				t.Errorf("List() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestChain_Handler(t *testing.T) {
	tests := []struct {
		name       string
		middleware []struct {
			name string
			fn   func(next http.Handler) http.Handler
		}
		finalResponse string
		wantResponse  string
	}{
		{
			name: "empty chain",
			middleware: []struct {
				name string
				fn   func(next http.Handler) http.Handler
			}{},
			finalResponse: "final",
			wantResponse:  "final",
		},
		{
			name: "single middleware",
			middleware: []struct {
				name string
				fn   func(next http.Handler) http.Handler
			}{
				{"middleware1", func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Write([]byte("1"))
						next.ServeHTTP(w, r)
					})
				}},
			},
			finalResponse: "final",
			wantResponse:  "1final",
		},
		{
			name: "multiple middleware",
			middleware: []struct {
				name string
				fn   func(next http.Handler) http.Handler
			}{
				{"middleware1", func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Write([]byte("1"))
						next.ServeHTTP(w, r)
					})
				}},
				{"middleware2", func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Write([]byte("2"))
						next.ServeHTTP(w, r)
					})
				}},
				{"middleware3", func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Write([]byte("3"))
						next.ServeHTTP(w, r)
					})
				}},
			},
			finalResponse: "final",
			wantResponse:  "123final",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := &Chain{}

			// Add middleware
			for _, m := range tt.middleware {
				chain.Append(m.name, m.fn)
			}

			// Create final handler
			finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(tt.finalResponse))
			})

			// Create chained handler
			chainedHandler := chain.Handler(finalHandler)

			// Test the handler
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			chainedHandler.ServeHTTP(w, req)

			got := w.Body.String()
			if diff := cmp.Diff(tt.wantResponse, got); diff != "" {
				t.Errorf("Handler() response mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestChain_HandlerExecutionOrder(t *testing.T) {
	tests := []struct {
		name       string
		middleware []struct {
			name string
			fn   func(next http.Handler) http.Handler
		}
		wantOrder []string
	}{
		{
			name: "single middleware with after logic",
			middleware: []struct {
				name string
				fn   func(next http.Handler) http.Handler
			}{
				{"first", func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						// This would be tracked in a real test
						next.ServeHTTP(w, r)
					})
				}},
			},
			wantOrder: []string{"first", "final", "first-after"},
		},
		{
			name: "multiple middleware with after logic",
			middleware: []struct {
				name string
				fn   func(next http.Handler) http.Handler
			}{
				{"first", func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						// This would be tracked in a real test
						next.ServeHTTP(w, r)
					})
				}},
				{"second", func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						// This would be tracked in a real test
						next.ServeHTTP(w, r)
					})
				}},
				{"third", func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						// This would be tracked in a real test
						next.ServeHTTP(w, r)
					})
				}},
			},
			wantOrder: []string{"first", "second", "third", "final", "third-after", "second-after", "first-after"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := &Chain{}
			executionOrder := []string{}

			// Add middleware that tracks execution order
			for _, m := range tt.middleware {
				middleware := m // capture loop variable
				chain.Append(middleware.name, func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						executionOrder = append(executionOrder, middleware.name)
						next.ServeHTTP(w, r)
						executionOrder = append(executionOrder, middleware.name+"-after")
					})
				})
			}

			// Create final handler
			finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				executionOrder = append(executionOrder, "final")
				w.Write([]byte("done"))
			})

			// Execute the chain
			chainedHandler := chain.Handler(finalHandler)
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			chainedHandler.ServeHTTP(w, req)

			if diff := cmp.Diff(tt.wantOrder, executionOrder); diff != "" {
				t.Errorf("Handler execution order mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestChain_ComplexOperations(t *testing.T) {
	tests := []struct {
		name       string
		operations func(*Chain)
		wantNames  []string
	}{
		{
			name: "complex chain manipulation",
			operations: func(chain *Chain) {
				// Build initial chain
				chain.Append("A", nil)
				chain.Append("B", nil)
				chain.Append("C", nil)

				// Insert before B
				if err := chain.InsertBefore("B", nil); err != nil {
					t.Fatal(err)
				}

				// Insert after C
				if err := chain.InsertAfter("C", nil); err != nil {
					t.Fatal(err)
				}

				// Remove A
				if err := chain.Remove("A"); err != nil {
					t.Fatal(err)
				}

				// Replace B
				if err := chain.Replace("B", nil); err != nil {
					t.Fatal(err)
				}
			},
			wantNames: []string{"inserted", "B", "C", "inserted"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := &Chain{}

			tt.operations(chain)

			got := chain.List()
			if diff := cmp.Diff(tt.wantNames, got); diff != "" {
				t.Errorf("Complex operations result mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
