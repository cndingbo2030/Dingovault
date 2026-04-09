package bus

import (
	"context"
	"testing"
)

func TestBusPublish(t *testing.T) {
	b := New()
	var got any
	b.Subscribe("t", func(_ context.Context, payload any) { got = payload })
	b.Publish(context.Background(), "t", 42)
	if got != 42 {
		t.Fatalf("got %v", got)
	}
}
