package session

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"
)

type codec interface {
	// Encode serializes the session data map
	Encode(sd persistedSession) ([]byte, error)

	// Decode deserializes the session data into a map
	Decode(data []byte) (persistedSession, error)
}

// gobCodec is a codec that uses Go's gob encoding
type gobCodec struct{}

var _ codec = (*gobCodec)(nil)

func init() {
	// register with a fixed name, so renames/refactors don't break existing
	// data.
	gob.RegisterName("github.com/lstoll/web/session.persistedSession", persistedSession{})
}

type flashLevel string

const (
	flashLevelNone  flashLevel = ""
	flashLevelInfo  flashLevel = "info"
	flashLevelError flashLevel = "error"
)

// persistedSession is the type that codecs are passed to serialize. Changes to
// this must be forward/backwards compatible. If we ever expose codec, we should
// think about stability beyond gob.
type persistedSession struct {
	Data      map[string]any
	CreatedAt time.Time
	UpdatedAt time.Time
	Flash     flashLevel
	FlashMsg  string
}

func (g *gobCodec) Encode(sess persistedSession) ([]byte, error) {
	var buf bytes.Buffer

	if err := gob.NewEncoder(&buf).Encode(sess); err != nil {
		return nil, fmt.Errorf("encoding session data: %w", err)
	}

	return buf.Bytes(), nil
}

func (g *gobCodec) Decode(data []byte) (persistedSession, error) {
	var result persistedSession

	err := gob.NewDecoder(bytes.NewReader(data)).Decode(&result)
	if err != nil {
		return persistedSession{}, fmt.Errorf("decoding session data: %w", err)
	}

	return result, nil
}
