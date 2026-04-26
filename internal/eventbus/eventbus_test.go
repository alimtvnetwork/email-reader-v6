// eventbus_test.go locks the three contracts the rest of the codebase
// relies on: (1) fan-out, (2) drop-on-slow, (3) unsubscribe + nil-safe.
package eventbus

import (
	"sync"
	"testing"
	"time"
)

type ev struct{ N int }

func TestBus_FanOutToAllSubscribers(t *testing.T) {
	b := New[ev](4)
	c1, cancel1 := b.Subscribe()
	c2, cancel2 := b.Subscribe()
	defer cancel1()
	defer cancel2()
	b.Publish(ev{N: 1})
	b.Publish(ev{N: 2})
	for _, ch := range []<-chan ev{c1, c2} {
		for want := 1; want <= 2; want++ {
			select {
			case got := <-ch:
				if got.N != want {
					t.Fatalf("want %d, got %d", want, got.N)
				}
			case <-time.After(time.Second):
				t.Fatal("timeout waiting for fan-out delivery")
			}
		}
	}
}

func TestBus_PublishDropsOnSlowSubscriber(t *testing.T) {
	b := New[ev](2)
	_, cancel := b.Subscribe()
	defer cancel()
	// 1000 publishes against a 2-buffer non-draining subscriber must
	// complete instantly (no stalls). We bound the test at 1s.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			b.Publish(ev{N: i})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish stalled on slow subscriber")
	}
}

func TestBus_UnsubscribeReleasesSlot(t *testing.T) {
	b := New[ev](1)
	_, cancel := b.Subscribe()
	if b.SubscriberCount() != 1 {
		t.Fatalf("want 1 subscriber, got %d", b.SubscriberCount())
	}
	cancel()
	if b.SubscriberCount() != 0 {
		t.Fatalf("want 0 after cancel, got %d", b.SubscriberCount())
	}
	// Double-cancel must not panic.
	cancel()
}

func TestBus_NilPublishIsNoop(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil Bus Publish panicked: %v", r)
		}
	}()
	var b *Bus[ev]
	b.Publish(ev{N: 42}) // must not panic
}

func TestBus_ConcurrentSubscribersAndPublishes(t *testing.T) {
	b := New[ev](16)
	var wg sync.WaitGroup
	subs := 8
	cancels := make([]func(), subs)
	chans := make([]<-chan ev, subs)
	for i := 0; i < subs; i++ {
		chans[i], cancels[i] = b.Subscribe()
	}
	defer func() {
		for _, c := range cancels {
			c()
		}
	}()
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			for j := 0; j < 8; j++ {
				b.Publish(ev{N: seed*100 + j})
			}
		}(i)
	}
	wg.Wait()
	// Every subscriber must have received SOMETHING (drops allowed,
	// total starvation is not).
	for i, ch := range chans {
		select {
		case <-ch:
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("subscriber %d received nothing", i)
		}
	}
}
