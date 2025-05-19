package internal

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TargetInterface is the very simple interface we're looking for.
type TargetInterface interface {
	http.ResponseWriter
	IsTarget() bool
	ID() string // To identify which instance was found
}

// Wrapper that implements TargetInterface and can be unwrapped.
type targetAndUnwrap struct {
	http.ResponseWriter
	name string
}

func (w *targetAndUnwrap) IsTarget() bool              { return true }
func (w *targetAndUnwrap) ID() string                  { return w.name }
func (w *targetAndUnwrap) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Wrapper that can be unwrapped but doesn't implement TargetInterface.
type onlyUnwrap struct {
	http.ResponseWriter
	name string
}

func (w *onlyUnwrap) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Wrapper that implements TargetInterface but CANNOT be unwrapped.
// If this is found, unwrapping stops.
type targetNoUnwrap struct {
	http.ResponseWriter
	name string
}

func (w *targetNoUnwrap) IsTarget() bool { return true }
func (w *targetNoUnwrap) ID() string     { return w.name }

// A plain http.ResponseWriter, acts as a base, does not implement TargetInterface or Unwrap.
type baseWriter struct {
	http.ResponseWriter
	name string
}

func TestFindResponseWriter(t *testing.T) {
	// A concrete, non-special http.ResponseWriter for the very end of chains
	concreteEnd := httptest.NewRecorder()
	plainBase := &baseWriter{ResponseWriter: concreteEnd, name: "plainBase"}

	// A base writer that itself implements the TargetInterface
	targetBase := &targetNoUnwrap{ResponseWriter: concreteEnd, name: "targetBase"}

	tests := []struct {
		name              string
		writerChain       http.ResponseWriter
		expectFound       bool
		expectedIDIfFound string
	}{
		{
			name:              "Target is the direct input (implements Target and Unwrap)",
			writerChain:       &targetAndUnwrap{ResponseWriter: plainBase, name: "directTargetUnwrap"},
			expectFound:       true,
			expectedIDIfFound: "directTargetUnwrap",
		},
		{
			name:              "Target is the direct input (implements Target, no Unwrap)",
			writerChain:       &targetNoUnwrap{ResponseWriter: plainBase, name: "directTargetNoUnwrap"},
			expectFound:       true,
			expectedIDIfFound: "directTargetNoUnwrap",
		},
		{
			name: "Target found after one unwrap",
			writerChain: &onlyUnwrap{
				ResponseWriter: &targetAndUnwrap{ResponseWriter: plainBase, name: "innerTarget"},
				name:           "outerUnwrap",
			},
			expectFound:       true,
			expectedIDIfFound: "innerTarget",
		},
		{
			name: "Target is deep in the chain",
			writerChain: &onlyUnwrap{
				ResponseWriter: &onlyUnwrap{
					ResponseWriter: &targetAndUnwrap{ResponseWriter: plainBase, name: "deepTarget"},
					name:           "middleUnwrap",
				},
				name: "outerUnwrap",
			},
			expectFound:       true,
			expectedIDIfFound: "deepTarget",
		},
		{
			name: "Target not found, chain unwraps to plain base",
			writerChain: &onlyUnwrap{
				ResponseWriter: &onlyUnwrap{
					ResponseWriter: plainBase,
					name:           "middleUnwrap",
				},
				name: "outerUnwrap",
			},
			expectFound: false,
		},
		{
			name: "Chain stops at non-unwrappable writer before a potential target",
			writerChain: &onlyUnwrap{
				ResponseWriter: &targetNoUnwrap{
					ResponseWriter: &targetAndUnwrap{ResponseWriter: plainBase, name: "hiddenTarget"},
					name:           "stopperTarget",
				},
				name: "outerUnwrap",
			},
			expectFound:       true,
			expectedIDIfFound: "stopperTarget",
		},
		{
			name: "Chain stops at non-unwrappable, non-target writer",
			writerChain: &onlyUnwrap{
				ResponseWriter: &baseWriter{
					ResponseWriter: &targetAndUnwrap{ResponseWriter: plainBase, name: "hiddenTarget"},
					name:           "stopperNonTarget",
				},
				name: "outerUnwrap",
			},
			expectFound: false,
		},
		{
			name:              "Input is a base writer that is the target",
			writerChain:       targetBase,
			expectFound:       true,
			expectedIDIfFound: "targetBase",
		},
		{
			name:        "Input is a plain base writer, not the target",
			writerChain: plainBase,
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			foundInstance, found := UnwrapResponseWriterTo[TargetInterface](tt.writerChain)

			if found != tt.expectFound {
				t.Errorf("FindResponseWriter() found = %v, want %v", found, tt.expectFound)
			}

			if tt.expectFound {
				if foundInstance == nil {
					t.Fatalf("Expected to find an instance of TargetInterface, but got nil")
				}
				if id := foundInstance.ID(); id != tt.expectedIDIfFound {
					t.Errorf("Expected found instance ID to be %q, but got %q", tt.expectedIDIfFound, id)
				}
				if !foundInstance.IsTarget() {
					t.Errorf("Found instance does not report IsTarget() == true")
				}
			} else {
				if foundInstance != nil {
					t.Errorf("Expected not to find an instance (should be nil), but got %T (ID: %s)",
						foundInstance, foundInstance.ID())
				}
			}
		})
	}
}
