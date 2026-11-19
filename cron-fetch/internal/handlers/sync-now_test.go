package handlers

import (
	"testing"
	"time"
)

func TestCooldownAllow(t *testing.T) {
	c := &cooldown{last: make(map[string]time.Time)}
	start := time.Now()

	if !c.allow("u1", start) {
		t.Fatal("first sync should be allowed")
	}
	if c.allow("u1", start.Add(30*time.Second)) {
		t.Error("second sync within the cooldown should be refused")
	}
	if !c.allow("u2", start.Add(30*time.Second)) {
		t.Error("another user shouldn't be held back by u1's cooldown")
	}
	if !c.allow("u1", start.Add(runNowCooldown)) {
		t.Error("sync after the cooldown should be allowed")
	}
}
