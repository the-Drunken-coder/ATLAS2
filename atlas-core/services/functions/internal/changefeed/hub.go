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
	mu     sync.Mutex
	ch     chan *sharedv1.MutationEvent
	err    error
	closed bool
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
	if s.closed {
		return
	}
	s.closed = true
	close(s.ch)
}

type Hub struct {
	mu          sync.Mutex
	nextID      int
	subscribers map[int]*Subscription
	logger      *slog.Logger
	done        chan struct{}
	closeOnce   sync.Once
	subWG       sync.WaitGroup
	closed      bool
}

func NewHub() *Hub {
	return NewHubWithLogger(slog.New(slog.NewTextHandler(os.Stderr, nil)))
}

func NewHubWithLogger(logger *slog.Logger) *Hub {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	return &Hub{
		subscribers: map[int]*Subscription{},
		logger:      logger,
		done:        make(chan struct{}),
	}
}

func (h *Hub) Publish(_ context.Context, event *sharedv1.MutationEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, sub := range h.subscribers {
		select {
		case sub.ch <- event:
		default:
			h.logger.Warn(ErrSubscriberEvicted.Error(),
				slog.String("component", "changefeed"),
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
	if h.closed {
		h.mu.Unlock()
		sub := &Subscription{ch: make(chan *sharedv1.MutationEvent)}
		sub.closeWithError(context.Canceled)
		return sub
	}
	id := h.nextID
	h.nextID++
	sub := &Subscription{ch: make(chan *sharedv1.MutationEvent, subscriberBufferSize)}
	h.subscribers[id] = sub
	h.subWG.Add(1)
	h.mu.Unlock()
	go func() {
		defer h.subWG.Done()
		select {
		case <-ctx.Done():
			h.mu.Lock()
			h.closeSubscriberLocked(id, ctx.Err())
			h.mu.Unlock()
		case <-h.done:
			h.mu.Lock()
			h.closeSubscriberLocked(id, context.Canceled)
			h.mu.Unlock()
		}
	}()
	return sub
}

func (h *Hub) Close() {
	h.closeOnce.Do(func() {
		h.mu.Lock()
		h.closed = true
		close(h.done)
		for id := range h.subscribers {
			h.closeSubscriberLocked(id, context.Canceled)
		}
		h.mu.Unlock()
		h.subWG.Wait()
	})
}

func (h *Hub) Done() <-chan struct{} {
	return h.done
}

func (h *Hub) closeSubscriberLocked(id int, err error) {
	sub, ok := h.subscribers[id]
	if !ok {
		return
	}
	delete(h.subscribers, id)
	sub.closeWithError(err)
}
