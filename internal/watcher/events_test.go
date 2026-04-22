package watcher

import (
	"sync"
	"testing"
	"time"
)

// TestBus_Subscribe verifies fan-out: every active subscriber receives every
// published event in the order they were sent.
func TestBus_Subscribe(t *testing.T) {
	b := NewBus(8)
	a, cancelA := b.Subscribe()
	c, cancelC := b.Subscribe()
	defer cancelA()
	defer cancelC()

	want := []EventKind{EventStarted, EventNewMail, EventStopped}
	for _, k := range want {
		b.Publish(Event{Kind: k, Alias: "x"})
	}

	collect := func(ch <-chan Event) []EventKind {
		got := make([]EventKind, 0, len(want))
		deadline := time.After(500 * time.Millisecond)
		for len(got) < len(want) {
			select {
			case ev := <-ch:
				got = append(got, ev.Kind)
			case <-deadline:
				return got
			}
		}
		return got
	}

	if g := collect(a); !equalKinds(g, want) {
		t.Fatalf("subscriber A got %v, want %v", g, want)
	}
	if g := collect(c); !equalKinds(g, want) {
		t.Fatalf("subscriber C got %v, want %v", g, want)
	}
}

// TestBus_Unsubscribe verifies cancel removes the subscriber and closes its
// channel so range-loops on the consumer side terminate.
func TestBus_Unsubscribe(t *testing.T) {
	b := NewBus(2)
	ch, cancel := b.Subscribe()
	if b.SubscriberCount() != 1 {
		t.Fatalf("want 1 subscriber, got %d", b.SubscriberCount())
	}
	cancel()
	if b.SubscriberCount() != 0 {
		t.Fatalf("want 0 subscribers after cancel, got %d", b.SubscriberCount())
	}
	if _, ok := <-ch; ok {
		t.Fatalf("expected channel closed after cancel")
	}
}

// TestBus_NonBlocking ensures a slow subscriber never stalls Publish — the
// watcher loop must keep running even if a UI consumer hangs.
func TestBus_NonBlocking(t *testing.T) {
	b := NewBus(1)
	_, cancel := b.Subscribe() // never read from
	defer cancel()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			b.Publish(Event{Kind: EventPollOK})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish stalled on a slow subscriber")
	}
}

// TestBus_NilSafe — a nil bus must accept Publish so the CLI can omit Options.Bus.
func TestBus_NilSafe(t *testing.T) {
	var b *Bus
	b.Publish(Event{Kind: EventStarted}) // must not panic
}

// TestBus_ConcurrentPublishSubscribe is a smoke test for races; run with -race.
func TestBus_ConcurrentPublishSubscribe(t *testing.T) {
	b := NewBus(64)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, cancel := b.Subscribe()
			defer cancel()
			for j := 0; j < 50; j++ {
				select {
				case <-ch:
				case <-time.After(20 * time.Millisecond):
				}
			}
		}()
	}
	for i := 0; i < 200; i++ {
		b.Publish(Event{Kind: EventPollOK})
	}
	wg.Wait()
}

func equalKinds(a, b []EventKind) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
