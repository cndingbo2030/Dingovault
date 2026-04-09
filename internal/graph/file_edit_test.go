package graph

import (
	"strings"
	"testing"
)

func TestReplaceBlockLineRange_preservesBullet(t *testing.T) {
	src := "intro\n  - old item\nrest\n"
	out, err := ReplaceBlockLineRange([]byte(src), 2, 2, "new item")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "  - new item") {
		t.Fatalf("got %q", out)
	}
	if !strings.Contains(string(out), "rest") {
		t.Fatalf("lost tail: %q", out)
	}
}

func TestReplaceBlockLineRange_heading(t *testing.T) {
	src := "# Old\n\nbody\n"
	out, err := ReplaceBlockLineRange([]byte(src), 1, 1, "New title")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(out), "# New title") {
		t.Fatalf("got %q", out)
	}
}
