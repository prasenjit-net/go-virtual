package scripting

import (
	"context"
	"testing"
	"time"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/storage"
)

// newEngineStore returns a MemoryStorage pre-populated with the given scripts and bindings.
func newEngineStore(t *testing.T, scripts []*models.Script, bindings []*models.ScriptBinding) storage.Storage {
	t.Helper()
	store := storage.NewMemoryStorage()
	for _, s := range scripts {
		if err := store.CreateScript(s); err != nil {
			t.Fatalf("CreateScript: %v", err)
		}
	}
	for _, b := range bindings {
		if err := store.CreateScriptBinding(b); err != nil {
			t.Fatalf("CreateScriptBinding: %v", err)
		}
	}
	return store
}

func makeScript(id, source string) *models.Script {
	return &models.Script{
		ID:        id,
		Name:      id,
		Source:    source,
		Timeout:   100,
		Enabled:   true,
		UpdatedAt: time.Now(),
	}
}

func makeBinding(id, opID, scriptID, outputKey string, order int) *models.ScriptBinding {
	return &models.ScriptBinding{
		ID:          id,
		OperationID: opID,
		ScriptID:    scriptID,
		OutputKey:   outputKey,
		Order:       order,
		Enabled:     true,
	}
}

func TestRunBindings_NoBindings(t *testing.T) {
	store := newEngineStore(t, nil, nil)
	engine := NewScriptEngine(store, 100)

	out, traces := engine.RunBindings(context.Background(), "op-1", &ScriptInput{})
	if len(out) != 0 {
		t.Errorf("Expected empty output, got %v", out)
	}
	if len(traces) != 0 {
		t.Errorf("Expected empty traces, got %v", traces)
	}
}

func TestRunBindings_SingleBinding(t *testing.T) {
	src := `
def run(req):
    return {"id": req["path"]["userId"], "ok": True}
`
	script := makeScript("s1", src)
	binding := makeBinding("b1", "op-1", "s1", "user", 0)

	store := newEngineStore(t, []*models.Script{script}, []*models.ScriptBinding{binding})
	engine := NewScriptEngine(store, 100)

	input := &ScriptInput{Path: map[string]string{"userId": "42"}}
	out, traces := engine.RunBindings(context.Background(), "op-1", input)

	if len(out) != 1 {
		t.Fatalf("Expected 1 output key, got %d: %v", len(out), out)
	}
	userOut, ok := out["user"].(map[string]any)
	if !ok {
		t.Fatalf("Expected map for 'user', got %T", out["user"])
	}
	if userOut["id"] != "42" {
		t.Errorf("id: got %v, want 42", userOut["id"])
	}
	if userOut["ok"] != true {
		t.Errorf("ok: got %v, want true", userOut["ok"])
	}

	if len(traces) != 1 {
		t.Fatalf("Expected 1 trace, got %d", len(traces))
	}
	if traces[0].OutputKey != "user" {
		t.Errorf("trace output key: got %q, want 'user'", traces[0].OutputKey)
	}
	if traces[0].Error != "" {
		t.Errorf("trace error: got %q, want empty", traces[0].Error)
	}
}

func TestRunBindings_MultipleBindingsOrdered(t *testing.T) {
	src1 := `def run(req): return "first"`
	src2 := `def run(req): return "second"`

	s1 := makeScript("s1", src1)
	s2 := makeScript("s2", src2)
	// b2 has lower order than b1 — should execute first but keyed under its own key
	b1 := makeBinding("b1", "op-1", "s1", "alpha", 10)
	b2 := makeBinding("b2", "op-1", "s2", "beta", 1)

	store := newEngineStore(t, []*models.Script{s1, s2}, []*models.ScriptBinding{b1, b2})
	engine := NewScriptEngine(store, 100)

	out, traces := engine.RunBindings(context.Background(), "op-1", &ScriptInput{})

	if len(out) != 2 {
		t.Fatalf("Expected 2 output keys, got %d", len(out))
	}
	if out["alpha"] != "first" {
		t.Errorf("alpha: got %v, want 'first'", out["alpha"])
	}
	if out["beta"] != "second" {
		t.Errorf("beta: got %v, want 'second'", out["beta"])
	}

	// traces length should be 2
	if len(traces) != 2 {
		t.Fatalf("Expected 2 traces, got %d", len(traces))
	}
}

func TestRunBindings_DisabledBindingSkipped(t *testing.T) {
	src := `def run(req): return "should not run"`
	script := makeScript("s1", src)
	binding := makeBinding("b1", "op-1", "s1", "result", 0)
	binding.Enabled = false

	store := newEngineStore(t, []*models.Script{script}, []*models.ScriptBinding{binding})
	engine := NewScriptEngine(store, 100)

	out, _ := engine.RunBindings(context.Background(), "op-1", &ScriptInput{})
	if len(out) != 0 {
		t.Errorf("Expected empty output for disabled binding, got %v", out)
	}
}

func TestRunBindings_DisabledScriptSkipped(t *testing.T) {
	src := `def run(req): return "should not run"`
	script := makeScript("s1", src)
	script.Enabled = false
	binding := makeBinding("b1", "op-1", "s1", "result", 0)

	store := newEngineStore(t, []*models.Script{script}, []*models.ScriptBinding{binding})
	engine := NewScriptEngine(store, 100)

	out, _ := engine.RunBindings(context.Background(), "op-1", &ScriptInput{})
	if len(out) != 0 {
		t.Errorf("Expected empty output for disabled script, got %v", out)
	}
}

func TestRunBindings_ScriptErrorDoesNotAbort(t *testing.T) {
	// s1 errors, s2 succeeds — s2's output must still be present
	bad := makeScript("s1", `def run(req): return 1 // 0`)
	good := makeScript("s2", `def run(req): return "ok"`)
	b1 := makeBinding("b1", "op-1", "s1", "bad", 0)
	b2 := makeBinding("b2", "op-1", "s2", "good", 1)

	store := newEngineStore(t, []*models.Script{bad, good}, []*models.ScriptBinding{b1, b2})
	engine := NewScriptEngine(store, 100)

	out, traces := engine.RunBindings(context.Background(), "op-1", &ScriptInput{})

	if _, ok := out["bad"]; ok {
		t.Error("Expected 'bad' key absent from output on error")
	}
	if out["good"] != "ok" {
		t.Errorf("good: got %v, want 'ok'", out["good"])
	}
	// Two traces: first with error, second without
	if len(traces) != 2 {
		t.Fatalf("Expected 2 traces, got %d", len(traces))
	}
	if traces[0].Error == "" {
		t.Error("Expected error in first trace")
	}
	if traces[1].Error != "" {
		t.Errorf("Expected no error in second trace, got %q", traces[1].Error)
	}
}

func TestRunBindings_CompilationErrorGraceful(t *testing.T) {
	script := makeScript("s1", `def run(req  # syntax error`)
	binding := makeBinding("b1", "op-1", "s1", "result", 0)

	store := newEngineStore(t, []*models.Script{script}, []*models.ScriptBinding{binding})
	engine := NewScriptEngine(store, 100)

	out, traces := engine.RunBindings(context.Background(), "op-1", &ScriptInput{})
	if len(out) != 0 {
		t.Errorf("Expected empty output for compile error, got %v", out)
	}
	if len(traces) != 1 || traces[0].Error == "" {
		t.Errorf("Expected 1 trace with error, got %+v", traces)
	}
}

func TestRunBindings_CacheHit(t *testing.T) {
	src := `def run(req): return "cached"`
	script := makeScript("s1", src)
	binding := makeBinding("b1", "op-1", "s1", "out", 0)

	store := newEngineStore(t, []*models.Script{script}, []*models.ScriptBinding{binding})
	engine := NewScriptEngine(store, 100)
	input := &ScriptInput{}

	// First call: compiles and caches
	out1, _ := engine.RunBindings(context.Background(), "op-1", input)
	// Second call: should hit cache
	out2, _ := engine.RunBindings(context.Background(), "op-1", input)

	if out1["out"] != "cached" || out2["out"] != "cached" {
		t.Errorf("Expected 'cached' from both calls, got %v, %v", out1, out2)
	}
}

func TestCompileAndValidate_Valid(t *testing.T) {
	store := newEngineStore(t, nil, nil)
	engine := NewScriptEngine(store, 100)

	err := engine.CompileAndValidate("s1", `def run(req): return {"ok": True}`)
	if err != nil {
		t.Errorf("Expected no error for valid source, got %v", err)
	}
}

func TestCompileAndValidate_Invalid(t *testing.T) {
	store := newEngineStore(t, nil, nil)
	engine := NewScriptEngine(store, 100)

	err := engine.CompileAndValidate("s1", `def run(req  # bad syntax`)
	if err == nil {
		t.Error("Expected error for invalid source")
	}
}

func TestTestScript_Success(t *testing.T) {
	store := newEngineStore(t, nil, nil)
	engine := NewScriptEngine(store, 100)

	script := makeScript("s1", `def run(req): return {"result": 42}`)
	result, durationMs, err := engine.TestScript(context.Background(), script, &ScriptInput{})
	if err != nil {
		t.Fatalf("TestScript error: %v", err)
	}
	if durationMs < 0 {
		t.Errorf("Expected non-negative duration, got %f", durationMs)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Expected map result, got %T", result)
	}
	if m["result"] != int64(42) {
		t.Errorf("result: got %v, want 42", m["result"])
	}
}

func TestTestScript_CompileError(t *testing.T) {
	store := newEngineStore(t, nil, nil)
	engine := NewScriptEngine(store, 100)

	script := makeScript("s1", `def run(req  # bad`)
	_, _, err := engine.TestScript(context.Background(), script, &ScriptInput{})
	if err == nil {
		t.Error("Expected error for invalid source")
	}
}

func TestTestScript_UsesDefaultTimeout(t *testing.T) {
	store := newEngineStore(t, nil, nil)
	engine := NewScriptEngine(store, 500) // 500ms default

	script := makeScript("s1", `def run(req): return "fast"`)
	script.Timeout = 0 // zero → use default

	result, _, err := engine.TestScript(context.Background(), script, &ScriptInput{})
	if err != nil {
		t.Fatalf("TestScript error: %v", err)
	}
	if result != "fast" {
		t.Errorf("Expected 'fast', got %v", result)
	}
}
