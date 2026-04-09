package bus

import (
	"context"
	"fmt"
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

func TestBeforeBlockSaveChain(t *testing.T) {
	b := New()
	b.RegisterBeforeBlockSave(func(_ context.Context, d BeforeBlockSaveData) (string, error) {
		return d.Content + "A", nil
	})
	b.RegisterBeforeBlockSave(func(_ context.Context, d BeforeBlockSaveData) (string, error) {
		return d.Content + "B", nil
	})
	out, err := b.BeforeBlockSave(context.Background(), BeforeBlockSaveData{Content: "0"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "0AB" {
		t.Fatalf("got %q", out)
	}
}

func TestBeforeBlockSaveAbort(t *testing.T) {
	b := New()
	b.RegisterBeforeBlockSave(func(_ context.Context, _ BeforeBlockSaveData) (string, error) {
		return "", fmt.Errorf("stop")
	})
	_, err := b.BeforeBlockSave(context.Background(), BeforeBlockSaveData{Content: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}
