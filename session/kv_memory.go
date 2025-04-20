package session

import (
	"context"
	"sync"
	"time"
)

type kvItem struct {
	data      []byte
	expiresAt time.Time
}

type memoryKV struct {
	contents   map[string]kvItem
	contentsMu sync.RWMutex
}

func NewMemoryKV() KV {
	return &memoryKV{contents: make(map[string]kvItem)}
}

func (m *memoryKV) Get(_ context.Context, key string) (_ []byte, found bool, _ error) {
	m.contentsMu.RLock()
	defer m.contentsMu.RUnlock()

	v, ok := m.contents[key]
	if !ok {
		return nil, false, nil
	}
	if time.Now().After(v.expiresAt) {
		delete(m.contents, key)
		return nil, false, nil
	}
	return v.data, true, nil
}

func (m *memoryKV) Set(_ context.Context, key string, expiresAt time.Time, value []byte) error {
	m.contentsMu.Lock()
	defer m.contentsMu.Unlock()

	m.contents[key] = kvItem{
		data:      value,
		expiresAt: expiresAt,
	}
	return nil
}

func (m *memoryKV) Delete(_ context.Context, key string) error {
	m.contentsMu.Lock()
	defer m.contentsMu.Unlock()

	delete(m.contents, key)
	return nil
}
