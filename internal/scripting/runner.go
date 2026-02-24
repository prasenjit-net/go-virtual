package scripting

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"go.starlark.net/starlark"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/store"
)

// CompiledScript is the interface for an immutable, pre-compiled Starlark program.
// Each call to Execute creates a fresh thread; the CompiledScript itself is safe
// for concurrent use.
type CompiledScript interface {
	// Execute runs the script with the given request input.
	// sess may be nil (Phase 1 behaviour — no store access).
	// When sess is non-nil, a `store` builtin is injected into the Starlark thread
	// and access events are appended to accessLog.
	// logBuf collects log() calls from the script; may be nil (logs are discarded).
	Execute(ctx context.Context, input *ScriptInput, timeoutMs int, sess *store.Session, accessLog *[]models.StoreAccessEvent, logBuf *[]string) (any, error)
}

// StarlarkRunner compiles and executes Starlark scripts.
type StarlarkRunner struct{}

// Compile parses and compiles a Starlark source string into a reusable CompiledScript.
// Returns a compile-time error (with line/column info) if the source is invalid.
func (r *StarlarkRunner) Compile(scriptID, source string) (CompiledScript, error) {
	filename := scriptID + ".star"
	_, prog, err := starlark.SourceProgram(filename, source, func(name string) bool {
		// Allow names injected at runtime: store, log, and all registered builtins
		return name == "store" || name == "log" || builtinNames[name]
	})
	if err != nil {
		return nil, fmt.Errorf("compile error: %w", err)
	}
	return &starlarkScript{prog: prog, filename: filename}, nil
}

// starlarkScript is a compiled Starlark program that can be executed multiple times.
type starlarkScript struct {
	prog     *starlark.Program
	filename string
}

// Execute runs the compiled script with the given request input and timeout.
// It calls the mandatory top-level `run(req)` function.
// When sess is non-nil a `store` Starlark builtin is injected for session store access.
// logBuf collects messages written via log() in the script; passing nil discards them.
func (s *starlarkScript) Execute(ctx context.Context, input *ScriptInput, timeoutMs int, sess *store.Session, accessLog *[]models.StoreAccessEvent, logBuf *[]string) (result any, err error) {
	// Recover from any Starlark panic
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("script panic: %v", r)
		}
	}()

	timeout := time.Duration(timeoutMs) * time.Millisecond

	thread := &starlark.Thread{
		Name: s.filename,
	}

	// Limit execution steps to guard against infinite loops (~10M steps ≈ seconds)
	thread.SetMaxExecutionSteps(10_000_000)

	// Apply wall-clock timeout via context cancellation
	done := make(chan struct{})
	if timeout > 0 {
		go func() {
			select {
			case <-time.After(timeout):
				thread.Cancel("timeout")
			case <-done:
			}
		}()
	}

	// Build predeclared names (available to the module as global constants)
	predeclared := starlark.StringDict{}
	if sess != nil {
		predeclared["store"] = store.NewStoreBuiltin(sess, accessLog)
	}
	// log(msg, ...) — always available; appends formatted message to logBuf.
	// When logBuf is nil (tracing disabled), calls are silently discarded.
	predeclared["log"] = starlark.NewBuiltin("log", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
		if logBuf != nil {
			parts := make([]string, 0, len(args))
			for _, arg := range args {
				parts = append(parts, arg.String())
			}
			*logBuf = append(*logBuf, strings.Join(parts, " "))
		}
		return starlark.None, nil
	})

	// Inject all standard builtins (uuid, now, rand_int, …)
	//nolint:gosec — script RNG does not need cryptographic quality
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	injectBuiltins(predeclared, rng, sess)

	globals, execErr := s.prog.Init(thread, predeclared)
	close(done)

	if execErr != nil {
		return nil, fmt.Errorf("init error: %w", execErr)
	}
	globals.Freeze()

	// Build the req dict
	reqDict := buildReqDict(input)

	// Look up the `run` function
	runFn, ok := globals["run"]
	if !ok {
		return nil, fmt.Errorf("script must define a top-level function named 'run'")
	}

	fn, ok := runFn.(*starlark.Function)
	if !ok {
		return nil, fmt.Errorf("'run' must be a function, got %T", runFn)
	}

	// Re-apply timeout for the actual call
	done2 := make(chan struct{})
	if timeout > 0 {
		thread2 := thread
		go func() {
			select {
			case <-time.After(timeout):
				thread2.Cancel("timeout")
			case <-done2:
			}
		}()
	}

	retVal, callErr := starlark.Call(thread, fn, starlark.Tuple{reqDict}, nil)
	close(done2)

	if callErr != nil {
		return nil, fmt.Errorf("runtime error: %w", callErr)
	}

	return StarToGo(retVal), nil
}

// buildReqDict constructs the Starlark dict passed to run(req).
func buildReqDict(input *ScriptInput) *starlark.Dict {
	req := new(starlark.Dict)

	// path
	pathDict := new(starlark.Dict)
	for k, v := range input.Path {
		_ = pathDict.SetKey(starlark.String(k), starlark.String(v))
	}
	_ = req.SetKey(starlark.String("path"), pathDict)

	// query
	queryDict := new(starlark.Dict)
	for k, v := range input.Query {
		_ = queryDict.SetKey(starlark.String(k), starlark.String(v))
	}
	_ = req.SetKey(starlark.String("query"), queryDict)

	// header
	headerDict := new(starlark.Dict)
	for k, v := range input.Header {
		_ = headerDict.SetKey(starlark.String(k), starlark.String(v))
	}
	_ = req.SetKey(starlark.String("header"), headerDict)

	// body
	_ = req.SetKey(starlark.String("body"), GoToStar(input.Body))

	return req
}
