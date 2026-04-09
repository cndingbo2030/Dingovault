package graph

import "testing"

func TestCycleTodoBody(t *testing.T) {
	if got := cycleTodoBody("buy"); got != "TODO buy" {
		t.Fatalf("none: %q", got)
	}
	if got := cycleTodoBody("TODO buy"); got != "DOING buy" {
		t.Fatalf("todo: %q", got)
	}
	if got := cycleTodoBody("DOING buy"); got != "DONE buy" {
		t.Fatalf("doing: %q", got)
	}
	if got := cycleTodoBody("DONE buy"); got != "buy" {
		t.Fatalf("done: %q", got)
	}
	if got := cycleTodoBody("done"); got != "" {
		t.Fatalf("done only: %q", got)
	}
}
