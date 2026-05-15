package scripting

import (
	"math/rand"
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

// execScript is a test helper that runs a Starlark script with all builtins
// injected and returns the result of calling run({}).
func execScript(t *testing.T, src string) starlark.Value {
	t.Helper()
	predeclared := starlark.StringDict{
		"store": newTestStore(),
	}
	rng := rand.New(rand.NewSource(42))
	injectBuiltins(predeclared, rng, nil)

	thread := &starlark.Thread{Name: "test"}
	globals, err := starlark.ExecFile(thread, "test.star", src, predeclared)
	if err != nil {
		t.Fatalf("ExecFile error: %v", err)
	}
	runFn, ok := globals["run"]
	if !ok {
		t.Fatal("script must define run()")
	}
	result, err := starlark.Call(thread, runFn, starlark.Tuple{starlark.NewDict(0)}, nil)
	if err != nil {
		t.Fatalf("run() error: %v", err)
	}
	return result
}

// mustString returns the result as a Go string, failing if it's not a string.
func mustString(t *testing.T, v starlark.Value) string {
	t.Helper()
	s, ok := v.(starlark.String)
	if !ok {
		t.Fatalf("expected string, got %s: %s", v.Type(), v)
	}
	return string(s)
}

// mustBool returns the result as a bool, failing if it's not.
func mustBool(t *testing.T, v starlark.Value) bool {
	t.Helper()
	b, ok := v.(starlark.Bool)
	if !ok {
		t.Fatalf("expected bool, got %s: %s", v.Type(), v)
	}
	return bool(b)
}

// newTestStore returns a simple in-memory store suitable for tests.
func newTestStore() starlark.Value {
	m := map[string]starlark.Value{}
	getBuiltin := starlark.NewBuiltin("get", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var key string
		var def starlark.Value
		if err := starlark.UnpackPositionalArgs("get", args, kwargs, 1, &key, &def); err != nil {
			return nil, err
		}
		if v, ok := m[key]; ok {
			return v, nil
		}
		if def != nil {
			return def, nil
		}
		return starlark.None, nil
	})
	setBuiltin := starlark.NewBuiltin("set", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var key string
		var val starlark.Value
		if err := starlark.UnpackPositionalArgs("set", args, kwargs, 2, &key, &val); err != nil {
			return nil, err
		}
		m[key] = val
		return starlark.None, nil
	})
	hasBuiltin := starlark.NewBuiltin("has", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var key string
		if err := starlark.UnpackPositionalArgs("has", args, kwargs, 1, &key); err != nil {
			return nil, err
		}
		_, ok := m[key]
		return starlark.Bool(ok), nil
	})
	d := starlark.NewDict(3)
	_ = d.SetKey(starlark.String("get"), getBuiltin)
	_ = d.SetKey(starlark.String("set"), setBuiltin)
	_ = d.SetKey(starlark.String("has"), hasBuiltin)

	// Wrap as a simple struct-like object via a dict — good enough for tests.
	// Real store is injected via injectBuiltins in production.
	return d
}

// ─── datetime module tests ───────────────────────────────────────────────────

func TestDatetime_TodayIsoformat(t *testing.T) {
	result := execScript(t, `
def run(req):
    today = datetime.date.today()
    return today.isoformat()
`)
	s := mustString(t, result)
	if len(s) != 10 || s[4] != '-' {
		t.Errorf("expected YYYY-MM-DD, got %q", s)
	}
}

func TestDatetime_DateConstructor(t *testing.T) {
	result := execScript(t, `
def run(req):
    d = datetime.date(2024, 3, 15)
    return d.isoformat()
`)
	s := mustString(t, result)
	if s != "2024-03-15" {
		t.Errorf("expected 2024-03-15, got %q", s)
	}
}

func TestDatetime_DateFromisoformat(t *testing.T) {
	result := execScript(t, `
def run(req):
    d = datetime.date.fromisoformat("2025-06-01")
    return d.isoformat()
`)
	if mustString(t, result) != "2025-06-01" {
		t.Errorf("fromisoformat round-trip failed")
	}
}

func TestDatetime_DateAttrs(t *testing.T) {
	result := execScript(t, `
def run(req):
    d = datetime.date(2024, 11, 28)
    return str(d.year) + "-" + str(d.month) + "-" + str(d.day)
`)
	if mustString(t, result) != "2024-11-28" {
		t.Errorf("date attrs wrong, got %q", mustString(t, result))
	}
}

// TestDatetime_SubtractDays covers the primary use-case from the user's original request.
func TestDatetime_SubtractDays(t *testing.T) {
	result := execScript(t, `
def run(req):
    today = datetime.date.fromisoformat("2025-01-10")
    three_days_ago = today - datetime.timedelta(days=3)
    return three_days_ago.isoformat()
`)
	if mustString(t, result) != "2025-01-07" {
		t.Errorf("expected 2025-01-07, got %q", mustString(t, result))
	}
}

func TestDatetime_AddDays(t *testing.T) {
	result := execScript(t, `
def run(req):
    d = datetime.date.fromisoformat("2025-01-10")
    future = d + datetime.timedelta(days=5)
    return future.isoformat()
`)
	if mustString(t, result) != "2025-01-15" {
		t.Errorf("expected 2025-01-15, got %q", mustString(t, result))
	}
}

func TestDatetime_DateDiff(t *testing.T) {
	result := execScript(t, `
def run(req):
    d1 = datetime.date.fromisoformat("2025-01-15")
    d2 = datetime.date.fromisoformat("2025-01-10")
    diff = d1 - d2
    return diff.days
`)
	n, ok := result.(starlark.Int)
	if !ok {
		t.Fatalf("expected int, got %T", result)
	}
	v, _ := n.Int64()
	if v != 5 {
		t.Errorf("expected diff.days = 5, got %d", v)
	}
}

func TestDatetime_UserExample(t *testing.T) {
	// Replicates the exact user-requested pattern from the original feature request.
	result := execScript(t, `
def run(req):
    today = datetime.date.today()
    three_days_ago = today - datetime.timedelta(days=3)
    return {
        "c_day": today.isoformat(),
        "c_day_minus_3": three_days_ago.isoformat(),
    }
`)
	d, ok := result.(*starlark.Dict)
	if !ok {
		t.Fatalf("expected dict, got %T", result)
	}
	today, _, _ := d.Get(starlark.String("c_day"))
	minus3, _, _ := d.Get(starlark.String("c_day_minus_3"))
	if today == nil || minus3 == nil {
		t.Fatal("missing keys in result")
	}
	todayStr := mustString(t, today)
	minus3Str := mustString(t, minus3)
	if len(todayStr) != 10 || len(minus3Str) != 10 {
		t.Errorf("expected YYYY-MM-DD dates, got %q and %q", todayStr, minus3Str)
	}
}

func TestDatetime_TimedeltaConstructor(t *testing.T) {
	result := execScript(t, `
def run(req):
    td = datetime.timedelta(days=1, hours=2, minutes=30)
    return td.total_seconds()
`)
	f, ok := result.(starlark.Float)
	if !ok {
		t.Fatalf("expected float, got %T", result)
	}
	expected := float64(1*24*3600 + 2*3600 + 30*60)
	if float64(f) != expected {
		t.Errorf("expected %v seconds, got %v", expected, f)
	}
}

func TestDatetime_TimedeltaComparison(t *testing.T) {
	result := execScript(t, `
def run(req):
    td1 = datetime.timedelta(days=1)
    td2 = datetime.timedelta(hours=24)
    return td1 == td2
`)
	if !mustBool(t, result) {
		t.Error("timedelta(days=1) should equal timedelta(hours=24)")
	}
}

func TestDatetime_DateComparison(t *testing.T) {
	result := execScript(t, `
def run(req):
    d1 = datetime.date.fromisoformat("2025-01-10")
    d2 = datetime.date.fromisoformat("2025-01-15")
    return d1 < d2
`)
	if !mustBool(t, result) {
		t.Error("2025-01-10 should be < 2025-01-15")
	}
}

func TestDatetime_DatetimeNow(t *testing.T) {
	result := execScript(t, `
def run(req):
    dt = datetime.datetime.now()
    return dt.isoformat()
`)
	s := mustString(t, result)
	if len(s) < 19 {
		t.Errorf("expected ISO datetime string, got %q", s)
	}
}

func TestDatetime_DatetimeConstructor(t *testing.T) {
	result := execScript(t, `
def run(req):
    dt = datetime.datetime(2024, 6, 15, hour=10, minute=30, second=0)
    return dt.isoformat()
`)
	s := mustString(t, result)
	if !strings.HasPrefix(s, "2024-06-15T10:30:00") {
		t.Errorf("expected 2024-06-15T10:30:00…, got %q", s)
	}
}

func TestDatetime_DatetimeFromisoformat(t *testing.T) {
	result := execScript(t, `
def run(req):
    dt = datetime.datetime.fromisoformat("2024-06-15T10:30:00Z")
    return str(dt.year) + "-" + str(dt.month) + "-" + str(dt.day)
`)
	if mustString(t, result) != "2024-6-15" {
		t.Errorf("unexpected fromisoformat result: %q", mustString(t, result))
	}
}

func TestDatetime_DatetimeSubtract(t *testing.T) {
	result := execScript(t, `
def run(req):
    dt1 = datetime.datetime.fromisoformat("2025-01-15T12:00:00Z")
    dt2 = datetime.datetime.fromisoformat("2025-01-10T12:00:00Z")
    diff = dt1 - dt2
    return diff.days
`)
	n, ok := result.(starlark.Int)
	if !ok {
		t.Fatalf("expected int, got %T", result)
	}
	v, _ := n.Int64()
	if v != 5 {
		t.Errorf("expected 5 days diff, got %d", v)
	}
}

func TestDatetime_DatetimeDateMethod(t *testing.T) {
	result := execScript(t, `
def run(req):
    dt = datetime.datetime(2024, 6, 15, hour=12)
    d = dt.date()
    return d.isoformat()
`)
	if mustString(t, result) != "2024-06-15" {
		t.Errorf("datetime.date() conversion failed")
	}
}

func TestDatetime_DateStrftime(t *testing.T) {
	result := execScript(t, `
def run(req):
    d = datetime.date(2025, 3, 5)
    return d.strftime("2006/01/02")
`)
	if mustString(t, result) != "2025/03/05" {
		t.Errorf("strftime failed, got %q", mustString(t, result))
	}
}

func TestDatetime_DateWeekday(t *testing.T) {
	// 2025-05-12 is a Monday → weekday() == 0
	result := execScript(t, `
def run(req):
    d = datetime.date.fromisoformat("2025-05-12")
    return d.weekday()
`)
	n, ok := result.(starlark.Int)
	if !ok {
		t.Fatalf("expected int, got %T", result)
	}
	v, _ := n.Int64()
	if v != 0 {
		t.Errorf("2025-05-12 is Monday, expected weekday()=0, got %d", v)
	}
}
