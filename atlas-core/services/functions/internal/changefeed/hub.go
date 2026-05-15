package changefeed

import (
	"context"
	"sync"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
)

type Hub struct {
	mu          sync.Mutex
	nextID      int
	subscribers map[int]chan *sharedv1.MutationEvent
}

func NewHub() *Hub {
	return &Hub{subscribers: map[int]chan *sharedv1.MutationEvent{}}
}

func (h *Hub) Publish(_ context.Context, event *sharedv1.MutationEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (h *Hub) Subscribe(ctx context.Context) <-chan *sharedv1.MutationEvent {
	h.mu.Lock()
	id := h.nextID
	h.nextID++
	ch := make(chan *sharedv1.MutationEvent, 32)
	h.subscribers[id] = ch
	h.mu.Unlock()
	go func() {
		<-ctx.Done()
		h.mu.Lock()
		delete(h.subscribers, id)
		close(ch)
		h.mu.Unlock()
	}()
	return ch
}
