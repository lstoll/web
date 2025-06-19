package middleware

import (
	"fmt"
	"net/http"
	"slices"
)

type chainedHandler struct {
	Name    string
	Handler func(next http.Handler) http.Handler
}

type Chain struct {
	handlers []*chainedHandler
}

func (c *Chain) Append(name string, handler func(next http.Handler) http.Handler) {
	c.handlers = append(c.handlers, &chainedHandler{Name: name, Handler: handler})
}

func (c *Chain) Prepend(name string, handler func(next http.Handler) http.Handler) {
	c.handlers = append([]*chainedHandler{{Name: name, Handler: handler}}, c.handlers...)
}

func (c *Chain) InsertBefore(name string, handler func(next http.Handler) http.Handler) error {
	for i, h := range c.handlers {
		if h.Name == name {
			insertedName := "inserted"
			c.handlers = append(c.handlers[:i], append([]*chainedHandler{{Name: insertedName, Handler: handler}}, c.handlers[i:]...)...)
			return nil
		}
	}
	return fmt.Errorf("handler %s not found", name)
}

func (c *Chain) InsertAfter(name string, handler func(next http.Handler) http.Handler) error {
	for i, h := range c.handlers {
		if h.Name == name {
			insertedName := "inserted"
			c.handlers = append(c.handlers[:i+1], append([]*chainedHandler{{Name: insertedName, Handler: handler}}, c.handlers[i+1:]...)...)
			return nil
		}
	}
	return fmt.Errorf("handler %s not found", name)
}

func (c *Chain) Remove(name string) error {
	for i, h := range c.handlers {
		if h.Name == name {
			c.handlers = slices.Delete(c.handlers, i, i+1)
			return nil
		}
	}
	return fmt.Errorf("handler %s not found", name)
}

func (c *Chain) Replace(name string, handler func(next http.Handler) http.Handler) error {
	for i, h := range c.handlers {
		if h.Name == name {
			c.handlers[i] = &chainedHandler{Name: name, Handler: handler}
			return nil
		}
	}
	return fmt.Errorf("handler %s not found", name)
}

func (c *Chain) List() []string {
	names := make([]string, len(c.handlers))
	for i, h := range c.handlers {
		names[i] = h.Name
	}
	return names
}

// Handler returns a new handler that applies the middleware chain to the
// provided handler.
func (c *Chain) Handler(h http.Handler) http.Handler {
	if c == nil || len(c.handlers) == 0 {
		return h
	}
	for i := len(c.handlers) - 1; i >= 0; i-- {
		h = c.handlers[i].Handler(h)
	}
	return h
}
