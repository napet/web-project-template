package events

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublishDeliversToTopicSubscribers(t *testing.T) {
	h := NewHub()
	a, unsubA := h.Subscribe("visitors")
	defer unsubA()
	b, unsubB := h.Subscribe("visitors")
	defer unsubB()

	h.Publish("visitors", Event{Type: "count", Data: 1})

	require.Equal(t, Event{Type: "count", Data: 1}, <-a)
	require.Equal(t, Event{Type: "count", Data: 1}, <-b)
}

func TestPublishIgnoresOtherTopics(t *testing.T) {
	h := NewHub()
	ch, unsub := h.Subscribe("visitors")
	defer unsub()

	h.Publish("other", Event{Type: "count"})

	select {
	case ev := <-ch:
		t.Fatalf("unexpected event %v", ev)
	default:
	}
}

func TestUnsubscribeClosesChannelAndRemovesTopic(t *testing.T) {
	h := NewHub()
	ch, unsub := h.Subscribe("visitors")
	require.Equal(t, 1, h.SubscriberCount("visitors"))

	unsub()

	_, ok := <-ch
	require.False(t, ok)
	require.Equal(t, 0, h.SubscriberCount("visitors"))
	// publishing to a topic without subscribers must not panic
	h.Publish("visitors", Event{Type: "count"})
}

func TestPublishDropsWhenBufferFull(t *testing.T) {
	h := NewHub()
	ch, unsub := h.Subscribe("visitors")
	defer unsub()

	for i := 0; i < subBufferSize+5; i++ {
		h.Publish("visitors", Event{Type: "count", Data: i})
	}

	require.Len(t, ch, subBufferSize)
	require.Equal(t, 0, (<-ch).Data)
}
