package events

import "sync"

type Handler interface {
	Handle(event Event) error
}

type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string][]Handler
}

func New() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]Handler),
	}
}

func (b *EventBus) Subscribe(eventType string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[eventType] = append(b.subscribers[eventType], handler)
}

func (b *EventBus) Publish(event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	handlers, ok := b.subscribers[event.Type()]
	if !ok {
		return
	}

	for _, handler := range handlers {
		go func(h Handler) { _ = h.Handle(event) }(handler)
	}
}
