package changefeed

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	sharedv1 "github.com/anomalyco/atlas-core/services/shared/gen/atlas/shared/v1"
)

func TestHubPublishDeliversEvents(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := hub.Subscribe(ctx)
	event := testMutationEvent("event-1")
	hub.Publish(context.Background(), event)

	select {
	case got, ok := <-sub.Events():
		if !ok {
			t.Fatal("subscription closed unexpectedly")
		}
		if got.GetEventId() != event.GetEventId() {
			t.Fatalf("expected event %q, got %q", event.GetEventId(), got.GetEventId())
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for published event")
	}
}

func TestHubEvictsLaggingSubscriberWhenBufferFills(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := hub.Subscribe(ctx)
	for i := 0; i <= subscriberBufferSize; i++ {
		hub.Publish(context.Background(), testMutationEvent(fmt.Sprintf("event-%d", i)))
	}

	drained := drainSubscription(t, sub)
	if len(drained) != subscriberBufferSize {
		t.Fatalf("expected %d buffered events before eviction, got %d", subscriberBufferSize, len(drained))
	}
	if !errors.Is(sub.Err(), ErrSubscriberEvicted) {
		t.Fatalf("expected ErrSubscriberEvicted, got %v", sub.Err())
	}
}

func TestHubCloseClosesAllSubscribers(t *testing.T) {
	hub := NewHub()
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	sub1 := hub.Subscribe(ctx1)
	sub2 := hub.Subscribe(ctx2)

	hub.Close()

	waitForSubscriptionClosed(t, sub1)
	waitForSubscriptionClosed(t, sub2)
	if !errors.Is(sub1.Err(), context.Canceled) {
		t.Fatalf("expected context.Canceled for sub1, got %v", sub1.Err())
	}
	if !errors.Is(sub2.Err(), context.Canceled) {
		t.Fatalf("expected context.Canceled for sub2, got %v", sub2.Err())
	}
}

func TestHubEvictsOnlySlowSubscriber(t *testing.T) {
	hub := NewHub()
	slowCtx, slowCancel := context.WithCancel(context.Background())
	defer slowCancel()
	fastCtx, fastCancel := context.WithCancel(context.Background())
	defer fastCancel()

	slow := hub.Subscribe(slowCtx)
	fast := hub.Subscribe(fastCtx)

	for i := 0; i <= subscriberBufferSize; i++ {
		event := testMutationEvent(fmt.Sprintf("event-%d", i))
		hub.Publish(context.Background(), event)

		select {
		case got, ok := <-fast.Events():
			if !ok {
				t.Fatal("fast subscription closed unexpectedly")
			}
			if got.GetEventId() != event.GetEventId() {
				t.Fatalf("expected fast subscriber to receive %q, got %q", event.GetEventId(), got.GetEventId())
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for fast subscriber event")
		}
	}

	if drained := drainSubscription(t, slow); len(drained) != subscriberBufferSize {
		t.Fatalf("expected slow subscriber to retain %d buffered events before eviction, got %d", subscriberBufferSize, len(drained))
	}
	if !errors.Is(slow.Err(), ErrSubscriberEvicted) {
		t.Fatalf("expected slow subscriber eviction, got %v", slow.Err())
	}

	extra := testMutationEvent("fast-extra")
	hub.Publish(context.Background(), extra)
	select {
	case got, ok := <-fast.Events():
		if !ok {
			t.Fatal("fast subscription closed unexpectedly after slow eviction")
		}
		if got.GetEventId() != extra.GetEventId() {
			t.Fatalf("expected extra event %q, got %q", extra.GetEventId(), got.GetEventId())
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for extra fast subscriber event")
	}
}

func testMutationEvent(id string) *sharedv1.MutationEvent {
	return &sharedv1.MutationEvent{
		EventId:    id,
		Resource:   "entity",
		Operation:  "updated",
		ResourceId: id,
	}
}

func drainSubscription(t *testing.T, sub *Subscription) []*sharedv1.MutationEvent {
	t.Helper()

	var events []*sharedv1.MutationEvent
	for {
		select {
		case event, ok := <-sub.Events():
			if !ok {
				return events
			}
			events = append(events, event)
		case <-time.After(time.Second):
			t.Fatal("timed out draining subscription")
		}
	}
}

func waitForSubscriptionClosed(t *testing.T, sub *Subscription) {
	t.Helper()

	select {
	case _, ok := <-sub.Events():
		if ok {
			t.Fatal("expected subscription to be closed")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscription close")
	}
}
