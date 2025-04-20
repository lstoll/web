package session

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"
)

// Session metadata keys with special prefix
const (
	MetadataCreatedAt = "__createdAt"
	MetadataUpdatedAt = "__updatedAt"
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
	enc := gob.NewEncoder(&buf)

	if err := enc.Encode(data); err != nil {
		return nil, fmt.Errorf("encoding session data: %w", err)
	}

	return buf.Bytes(), nil
}

func (g *gobCodec) Decode(data []byte) (map[string]any, error) {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)

	var result map[string]any
	if err := dec.Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding session data: %w", err)
	}

	return result, nil
}

// extractMetadata gets the metadata from the session map
func extractMetadata(data map[string]any) *sessionMetadata {
	var md sessionMetadata

	if createdAt, ok := data[MetadataCreatedAt].(time.Time); ok {
		md.CreatedAt = createdAt
	}

	if updatedAt, ok := data[MetadataUpdatedAt].(time.Time); ok {
		md.UpdatedAt = updatedAt
	}

	return &md
}

// setMetadata stores the metadata in the session map
func setMetadata(data map[string]any, md *sessionMetadata) {
	data[MetadataCreatedAt] = md.CreatedAt
	data[MetadataUpdatedAt] = md.UpdatedAt
}
