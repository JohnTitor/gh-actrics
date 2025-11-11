package cache

import (
	"testing"
	"time"
)

func TestCacheSetGet(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir, time.Minute)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	if _, ok, err := c.Get("missing"); err != nil || ok {
		t.Fatalf("expected miss without error, got ok=%v err=%v", ok, err)
	}

	data := []byte("value")
	if err := c.Set("key", data); err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	got, ok, err := c.Get("key")
	if err != nil {
		t.Fatalf("failed to get cache: %v", err)
	}
	if !ok {
		t.Fatalf("expected cache hit")
	}
	if string(got) != string(data) {
		t.Fatalf("expected %q, got %q", data, got)
	}
}

func TestCacheExpires(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	if err := c.Set("key", []byte("value")); err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	time.Sleep(30 * time.Millisecond)

	if _, ok, err := c.Get("key"); err != nil {
		t.Fatalf("get failed: %v", err)
	} else if ok {
		t.Fatalf("expected cache miss after expiration")
	}
}
