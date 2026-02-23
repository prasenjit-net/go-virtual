package scripting

import (
	"testing"
	"time"
)

func newMockScript(id string) CompiledScript {
	runner := &StarlarkRunner{}
	src := `def run(req): return "` + id + `"`
	cs, _ := runner.Compile(id, src)
	return cs
}

func TestCache_MissOnEmpty(t *testing.T) {
	c := newCompiledCache()
	_, ok := c.Get("script-1", time.Now())
	if ok {
		t.Error("Expected cache miss on empty cache")
	}
}

func TestCache_HitAfterSet(t *testing.T) {
	c := newCompiledCache()
	ts := time.Now()
	cs := newMockScript("test")
	c.Set("script-1", ts, cs)

	got, ok := c.Get("script-1", ts)
	if !ok {
		t.Fatal("Expected cache hit after Set")
	}
	if got == nil {
		t.Error("Expected non-nil CompiledScript from cache")
	}
}

func TestCache_MissOnTimestampMismatch(t *testing.T) {
	c := newCompiledCache()
	ts1 := time.Now()
	ts2 := ts1.Add(time.Second)
	cs := newMockScript("test")
	c.Set("script-1", ts1, cs)

	_, ok := c.Get("script-1", ts2)
	if ok {
		t.Error("Expected cache miss when updatedAt differs")
	}
}

func TestCache_MissAfterDelete(t *testing.T) {
	c := newCompiledCache()
	ts := time.Now()
	cs := newMockScript("test")
	c.Set("script-1", ts, cs)
	c.Delete("script-1")

	_, ok := c.Get("script-1", ts)
	if ok {
		t.Error("Expected cache miss after Delete")
	}
}

func TestCache_MultipleEntries(t *testing.T) {
	c := newCompiledCache()
	ts := time.Now()
	cs1 := newMockScript("one")
	cs2 := newMockScript("two")
	c.Set("s1", ts, cs1)
	c.Set("s2", ts, cs2)

	if _, ok := c.Get("s1", ts); !ok {
		t.Error("Expected hit for s1")
	}
	if _, ok := c.Get("s2", ts); !ok {
		t.Error("Expected hit for s2")
	}
	if _, ok := c.Get("s3", ts); ok {
		t.Error("Expected miss for s3 (not stored)")
	}
}

func TestCache_OverwriteEntry(t *testing.T) {
	c := newCompiledCache()
	ts1 := time.Now()
	ts2 := ts1.Add(time.Millisecond)
	cs1 := newMockScript("v1")
	cs2 := newMockScript("v2")

	c.Set("script-1", ts1, cs1)
	c.Set("script-1", ts2, cs2)

	// old ts1 should miss now
	if _, ok := c.Get("script-1", ts1); ok {
		t.Error("Expected miss for old timestamp after overwrite")
	}
	// new ts2 should hit
	if _, ok := c.Get("script-1", ts2); !ok {
		t.Error("Expected hit for new timestamp after overwrite")
	}
}
