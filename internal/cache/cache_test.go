package cache

import (
	"testing"
	"time"
)

func TestCacheEvictsOldestEntryWhenMaxSizeExceeded(t *testing.T) {
	c := New(2)

	c.Set("first", "a", time.Hour)
	c.Set("second", "b", time.Hour)
	c.Set("third", "c", time.Hour)

	if _, found := c.Get("first"); found {
		t.Fatal("expected oldest cache entry to be evicted")
	}
	if got, found := c.Get("second"); !found || got != "b" {
		t.Fatalf("expected second cache entry to remain, got value=%v found=%v", got, found)
	}
	if got, found := c.Get("third"); !found || got != "c" {
		t.Fatalf("expected newest cache entry to remain, got value=%v found=%v", got, found)
	}
}
