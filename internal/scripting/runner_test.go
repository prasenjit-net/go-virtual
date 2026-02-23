package scripting

import (
	"context"
	"strings"
	"testing"
)

func TestCompile_Valid(t *testing.T) {
	runner := &StarlarkRunner{}
	src := `
def run(req):
    return {"ok": True}
`
	cs, err := runner.Compile("test-script", src)
	if err != nil {
		t.Fatalf("Compile returned unexpected error: %v", err)
	}
	if cs == nil {
		t.Fatal("Compile returned nil CompiledScript")
	}
}

func TestCompile_SyntaxError(t *testing.T) {
	runner := &StarlarkRunner{}
	src := `def run(req  # missing closing paren`
	_, err := runner.Compile("bad-script", src)
	if err == nil {
		t.Fatal("Expected compile error for invalid syntax, got nil")
	}
	if !strings.Contains(err.Error(), "compile error") {
		t.Errorf("Expected 'compile error' in message, got: %v", err)
	}
}

func TestExecute_ReturnsDict(t *testing.T) {
	runner := &StarlarkRunner{}
	src := `
def run(req):
    return {
        "userId": req["path"]["id"],
        "format": req["query"]["fmt"],
        "auth":   req["header"]["authorization"],
    }
`
	cs, err := runner.Compile("s1", src)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	input := &ScriptInput{
		Path:   map[string]string{"id": "42"},
		Query:  map[string]string{"fmt": "json"},
		Header: map[string]string{"authorization": "Bearer tok"},
	}
	result, err := cs.Execute(context.Background(), input, 100)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Expected map[string]any, got %T", result)
	}
	if m["userId"] != "42" {
		t.Errorf("userId: got %v, want 42", m["userId"])
	}
	if m["format"] != "json" {
		t.Errorf("format: got %v, want json", m["format"])
	}
}

func TestExecute_ReturnsString(t *testing.T) {
	runner := &StarlarkRunner{}
	src := `def run(req): return "hello"`
	cs, _ := runner.Compile("s2", src)
	result, err := cs.Execute(context.Background(), &ScriptInput{}, 100)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != "hello" {
		t.Errorf("Expected 'hello', got %v", result)
	}
}

func TestExecute_ReturnsBool(t *testing.T) {
	runner := &StarlarkRunner{}
	src := `def run(req): return True`
	cs, _ := runner.Compile("s3", src)
	result, err := cs.Execute(context.Background(), &ScriptInput{}, 100)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != true {
		t.Errorf("Expected true, got %v", result)
	}
}

func TestExecute_ReturnsInt(t *testing.T) {
	runner := &StarlarkRunner{}
	src := `def run(req): return 42`
	cs, _ := runner.Compile("s4", src)
	result, err := cs.Execute(context.Background(), &ScriptInput{}, 100)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != int64(42) {
		t.Errorf("Expected int64(42), got %v (%T)", result, result)
	}
}

func TestExecute_NoRunFunction(t *testing.T) {
	runner := &StarlarkRunner{}
	src := `x = 1`
	cs, err := runner.Compile("s5", src)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	_, err = cs.Execute(context.Background(), &ScriptInput{}, 100)
	if err == nil {
		t.Fatal("Expected error for missing run function")
	}
}

func TestExecute_RuntimeError(t *testing.T) {
	runner := &StarlarkRunner{}
	src := `
def run(req):
    x = req["path"]["missing_key"]  # key access on empty dict - ok, returns None in Starlark
    return 1 // 0  # division by zero
`
	cs, err := runner.Compile("s6", src)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	_, err = cs.Execute(context.Background(), &ScriptInput{
		Path: map[string]string{},
	}, 100)
	if err == nil {
		t.Fatal("Expected runtime error for division by zero")
	}
}

func TestExecute_Arithmetic(t *testing.T) {
	runner := &StarlarkRunner{}
	src := `
def run(req):
    qty   = 3
    price = 12.5
    total = qty * price
    return {
        "total":  total,
        "taxed":  total * 1.2,
        "free":   total > 50,
    }
`
	cs, _ := runner.Compile("s7", src)
	result, err := cs.Execute(context.Background(), &ScriptInput{}, 100)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	m := result.(map[string]any)
	taxed, ok := m["taxed"].(float64)
	if !ok {
		t.Fatalf("taxed not float64, got %T", m["taxed"])
	}
	if taxed < 44.9 || taxed > 45.1 {
		t.Errorf("taxed: got %v, want ~45.0", taxed)
	}
	if m["free"] != false {
		t.Errorf("free: got %v, want false", m["free"])
	}
}

func TestExecute_BodyAccess(t *testing.T) {
	runner := &StarlarkRunner{}
	src := `
def run(req):
    name = req["body"]["name"]
    return "Hello, " + name
`
	cs, _ := runner.Compile("s8", src)
	result, err := cs.Execute(context.Background(), &ScriptInput{
		Body: map[string]any{"name": "Alice"},
	}, 100)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != "Hello, Alice" {
		t.Errorf("Expected 'Hello, Alice', got %v", result)
	}
}

func TestExecute_Timeout(t *testing.T) {
	runner := &StarlarkRunner{}
	// Infinite loop — should be cancelled by timeout
	src := `
def run(req):
    i = 0
    for _ in range(1000000000):
        i = i + 1
    return i
`
	cs, err := runner.Compile("s9", src)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	_, err = cs.Execute(context.Background(), &ScriptInput{}, 5)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
}
