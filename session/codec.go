package session

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"
)

// Session metadata keys for persisting in the map
const (
	metadataCreatedAt = "__createdAt"
	metadataUpdatedAt = "__updatedAt"
)

type codec interface {
	// Encode serializes the session data map
	Encode(data map[string]any) ([]byte, error)

	// Decode deserializes the session data into a map
	Decode(data []byte) (map[string]any, error)
}

// gobCodec is a codec that uses Go's gob encoding
type gobCodec struct{}

var _ codec = (*gobCodec)(nil)

func init() {
	// Register common types with gob
	gob.Register(time.Time{})
	gob.Register(map[string]any{})
	gob.Register([]interface{}{})
	gob.Register(map[string]interface{}{})
}

func (g *gobCodec) Encode(data map[string]any) ([]byte, error) {
	var buf bytes.Buffer

	if err := gob.NewEncoder(&buf).Encode(data); err != nil {
		return nil, fmt.Errorf("encoding session data: %w", err)
	}

	return buf.Bytes(), nil
}

func (g *gobCodec) Decode(data []byte) (map[string]any, error) {
	var result map[string]any
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding session data: %w", err)
	}

	return result, nil
}

// extractMetadata gets the metadata from the session map
func extractMetadata(data map[string]any) *sessionMetadata {
	var md sessionMetadata

	if createdAt, ok := data[metadataCreatedAt].(time.Time); ok {
		md.CreatedAt = createdAt
	}

	if updatedAt, ok := data[metadataUpdatedAt].(time.Time); ok {
		md.UpdatedAt = updatedAt
	}

	return &md
}

// setMetadata stores the metadata in the session map
func setMetadata(data map[string]any, md *sessionMetadata) {
	data[metadataCreatedAt] = md.CreatedAt
	data[metadataUpdatedAt] = md.UpdatedAt
}
