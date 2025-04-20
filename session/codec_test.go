package session

import (
	"testing"
)

func TestGobEncoding(t *testing.T) {
	// Create a sample data map similar to what's used in the E2E test
	data := map[string]any{
		"test0": "value0",
		"test1": "value1",
	}

	// Add session metadata
	md := &sessionMetadata{}
	setMetadata(data, md)

	// Encode the data
	g := &gobCodec{}
	encodedData, err := g.Encode(data)
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	// Decode using the same codec
	decodedData, err := g.Decode(encodedData)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	// Check if values match
	if decodedData["test0"] != "value0" || decodedData["test1"] != "value1" {
		t.Fatalf("Data mismatch: %v", decodedData)
	}
}
