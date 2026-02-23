package scripting

import (
	"context"
	"time"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/storage"
)

const defaultTimeoutMs = 100

// ScriptEngine manages script compilation, caching, and execution for the proxy pipeline.
type ScriptEngine struct {
	store            storage.Storage
	runner           *StarlarkRunner
	cache            *compiledCache
	defaultTimeoutMs int
}

// NewScriptEngine creates a ScriptEngine backed by the given storage.
// defaultTimeout controls the execution timeout per script (in milliseconds) when
// a script has Timeout = 0.
func NewScriptEngine(store storage.Storage, defaultTimeout int) *ScriptEngine {
	if defaultTimeout <= 0 {
		defaultTimeout = defaultTimeoutMs
	}
	return &ScriptEngine{
		store:            store,
		runner:           &StarlarkRunner{},
		cache:            newCompiledCache(),
		defaultTimeoutMs: defaultTimeout,
	}
}

// RunBindings executes all enabled script bindings for an operation in Order sequence.
// It returns:
//   - a map of outputKey → script result (empty map if no bindings or all failed)
//   - a slice of ScriptTrace records for tracing/debugging
//
// Errors in individual scripts are captured in the trace and do not abort execution.
func (e *ScriptEngine) RunBindings(
	ctx context.Context,
	operationID string,
	input *ScriptInput,
) (map[string]any, []models.ScriptTrace) {
	output := make(map[string]any)
	var traces []models.ScriptTrace

	bindings, err := e.store.GetScriptBindings(operationID)
	if err != nil || len(bindings) == 0 {
		return output, traces
	}

	for _, binding := range bindings {
		if !binding.Enabled {
			continue
		}

		script, err := e.store.GetScript(binding.ScriptID)
		if err != nil || script == nil || !script.Enabled {
			continue
		}

		st := models.ScriptTrace{
			BindingID:  binding.ID,
			ScriptID:   script.ID,
			ScriptName: script.Name,
			OutputKey:  binding.OutputKey,
		}

		// Resolve or compile the script
		compiled, cacheHit := e.cache.Get(script.ID, script.UpdatedAt)
		if !cacheHit {
			var compileErr error
			compiled, compileErr = e.runner.Compile(script.ID, script.Source)
			if compileErr != nil {
				st.Error = compileErr.Error()
				traces = append(traces, st)
				continue
			}
			e.cache.Set(script.ID, script.UpdatedAt, compiled)
		}

		// Determine timeout
		timeoutMs := script.Timeout
		if timeoutMs <= 0 {
			timeoutMs = e.defaultTimeoutMs
		}

		start := time.Now()
		result, execErr := compiled.Execute(ctx, input, timeoutMs)
		st.DurationMs = float64(time.Since(start).Microseconds()) / 1000.0

		if execErr != nil {
			st.Error = execErr.Error()
		} else {
			st.Output = result
			output[binding.OutputKey] = result
		}

		traces = append(traces, st)
	}

	return output, traces
}

// CompileAndValidate compiles a script source without caching or executing it.
// Used for /validate endpoint. Returns a compile error string, or "" if valid.
func (e *ScriptEngine) CompileAndValidate(scriptID, source string) error {
	_, err := e.runner.Compile(scriptID, source)
	return err
}

// TestScript compiles (or uses cache) and executes a script with a provided input.
// Used for the /test endpoint.
func (e *ScriptEngine) TestScript(
	ctx context.Context,
	script *models.Script,
	input *ScriptInput,
) (any, float64, error) {
	compiled, cacheHit := e.cache.Get(script.ID, script.UpdatedAt)
	if !cacheHit {
		var err error
		compiled, err = e.runner.Compile(script.ID, script.Source)
		if err != nil {
			return nil, 0, err
		}
		e.cache.Set(script.ID, script.UpdatedAt, compiled)
	}

	timeoutMs := script.Timeout
	if timeoutMs <= 0 {
		timeoutMs = e.defaultTimeoutMs
	}

	start := time.Now()
	result, err := compiled.Execute(ctx, input, timeoutMs)
	durationMs := float64(time.Since(start).Microseconds()) / 1000.0

	return result, durationMs, err
}
