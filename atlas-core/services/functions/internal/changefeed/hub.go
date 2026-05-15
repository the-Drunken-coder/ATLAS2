package changefeed

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
)

const subscriberEvictedMessage = "changefeed subscriber fell behind; refetch full state and resubscribe"

var ErrSubscriberEvicted = errors.New(subscriberEvictedMessage)

const subscriberBufferSize = 32

type Subscription struct {
	mu  sync.Mutex
	ch  chan *sharedv1.MutationEvent
	err error
}

func (s *Subscription) Events() <-chan *sharedv1.MutationEvent {
	return s.ch
}

func (s *Subscription) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *Subscription) closeWithError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		s.err = err
	}
	close(s.ch)
}

type Hub struct {
	mu          sync.Mutex
	nextID      int
	subscribers map[int]*Subscription
	logger      *slog.Logger
}

func NewHub() *Hub {
	return &Hub{
		subscribers: map[int]*Subscription{},
		logger:      slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}
}

func (h *Hub) Publish(_ context.Context, event *sharedv1.MutationEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, sub := range h.subscribers {
		select {
		case sub.ch <- event:
		default:
			h.logger.Warn("changefeed", ErrSubscriberEvicted.Error(),
				slog.Int("subscriber_id", id),
				slog.String("resource", event.GetResource()),
				slog.String("operation", event.GetOperation()),
			)
			h.closeSubscriberLocked(id, ErrSubscriberEvicted)
		}
	}
}

func (h *Hub) Subscribe(ctx context.Context) *Subscription {
	h.mu.Lock()
	id := h.nextID
	h.nextID++
	sub := &Subscription{ch: make(chan *sharedv1.MutationEvent, subscriberBufferSize)}
	h.subscribers[id] = sub
	h.mu.Unlock()
	go func() {
		<-ctx.Done()
		h.mu.Lock()
		h.closeSubscriberLocked(id, ctx.Err())
		h.mu.Unlock()
	}()
	return sub
}

func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id := range h.subscribers {
		h.closeSubscriberLocked(id, context.Canceled)
	}
}

func (h *Hub) closeSubscriberLocked(id int, err error) {
	sub, ok := h.subscribers[id]
	if !ok {
		return
	}
	delete(h.subscribers, id)
	sub.closeWithError(err)
}
