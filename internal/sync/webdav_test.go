package vaultsync

import (
	"testing"
	"time"
)

func TestClassifySync(t *testing.T) {
	t0 := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(10 * time.Minute)
	local := &fileSnapshot{modTime: t0, size: 10}
	remote := &fileSnapshot{modTime: t0, size: 10}
	if classifySync(local, remote) != syncSkip {
		t.Fatalf("equal snap should skip")
	}
	if classifySync(nil, remote) != syncPull {
		t.Fatalf("missing local -> pull")
	}
	if classifySync(local, nil) != syncPush {
		t.Fatalf("missing remote -> push")
	}
	if classifySync(&fileSnapshot{modTime: t1, size: 9}, &fileSnapshot{modTime: t0, size: 10}) != syncConflict {
		t.Fatalf("time+size both differ -> conflict")
	}
	if classifySync(&fileSnapshot{modTime: t1, size: 10}, &fileSnapshot{modTime: t0, size: 10}) != syncPush {
		t.Fatalf("newer local same size -> push")
	}
	if classifySync(&fileSnapshot{modTime: t0, size: 10}, &fileSnapshot{modTime: t1, size: 10}) != syncPull {
		t.Fatalf("newer remote same size -> pull")
	}
}
