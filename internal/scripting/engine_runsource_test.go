package scripting

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/prasenjit/go-virtual/internal/store"
)

func TestRunSource_SuccessUsesEphemeralStore(t *testing.T) {
	backend := newEngineStore(t, nil, nil)
	engine := NewScriptEngine(backend, 250)

	gs, err := store.NewGlobalStore(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatalf("NewGlobalStore: %v", err)
	}
	if err := gs.Set("seed", "hello"); err != nil {
		t.Fatalf("Set(seed): %v", err)
	}
	engine.SetGlobalStore(gs)

	src := `

def run(req):
    log("running")
    store.set("seed", "changed")
    return {"seed": store.get("seed"), "path": req.path("id", "")}
`
	result, logs, durationMs, err := engine.RunSource(context.Background(), src, 0, &ScriptInput{Path: map[string]string{"id": "42"}})
	if err != nil {
		t.Fatalf("RunSource error: %v", err)
	}
	if durationMs < 0 {
		t.Fatalf("expected non-negative duration, got %f", durationMs)
	}
	if len(logs) != 1 || logs[0] != `"running"` {
		t.Fatalf("unexpected logs: %#v", logs)
	}
	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map output, got %T", result)
	}
	if out["seed"] != "changed" || out["path"] != "42" {
		t.Fatalf("unexpected output: %#v", out)
	}
	if persisted, _ := gs.Get("seed"); persisted != "hello" {
		t.Fatalf("expected global store to remain unchanged, got %v", persisted)
	}
}

func TestRunSource_CompileError(t *testing.T) {
	engine := NewScriptEngine(newEngineStore(t, nil, nil), 100)
	_, _, _, err := engine.RunSource(context.Background(), `def run(req  # bad`, 100, &ScriptInput{})
	if err == nil {
		t.Fatal("expected compile error")
	}
}
