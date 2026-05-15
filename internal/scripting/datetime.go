package scripting

import (
	"fmt"
	"strings"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"
)

// ─── module constructor ───────────────────────────────────────────────────────

// newDatetimeModule returns the "datetime" module exposing date, datetime, and
// timedelta constructors in a style familiar to Python users.
func newDatetimeModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "datetime",
		Members: starlark.StringDict{
			"date":      dateType{},
			"datetime":  datetimeType{},
			"timedelta": timedeltaType{},
		},
	}
}

// ─── timedelta ────────────────────────────────────────────────────────────────

// timedeltaValue represents a duration (days, hours, minutes, seconds).
// Backed by time.Duration for precision.
type timedeltaValue struct {
	d time.Duration
}

var _ starlark.Value = (*timedeltaValue)(nil)
var _ starlark.HasAttrs = (*timedeltaValue)(nil)
var _ starlark.HasBinary = (*timedeltaValue)(nil)
var _ starlark.Comparable = (*timedeltaValue)(nil)

func (v *timedeltaValue) String() string {
	days := int(v.d.Hours() / 24)
	rem := v.d - time.Duration(days)*24*time.Hour
	h := int(rem.Hours())
	rem -= time.Duration(h) * time.Hour
	m := int(rem.Minutes())
	rem -= time.Duration(m) * time.Minute
	s := int(rem.Seconds())
	if days != 0 {
		return fmt.Sprintf("timedelta(days=%d, hours=%d, minutes=%d, seconds=%d)", days, h, m, s)
	}
	if h != 0 {
		return fmt.Sprintf("timedelta(hours=%d, minutes=%d, seconds=%d)", h, m, s)
	}
	return fmt.Sprintf("timedelta(minutes=%d, seconds=%d)", m, s)
}
func (v *timedeltaValue) Type() string          { return "timedelta" }
func (v *timedeltaValue) Freeze()               {}
func (v *timedeltaValue) Truth() starlark.Bool  { return v.d != 0 }
func (v *timedeltaValue) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: timedelta") }

func (v *timedeltaValue) Attr(name string) (starlark.Value, error) {
	switch name {
	case "days":
		return starlark.MakeInt(int(v.d.Hours() / 24)), nil
	case "hours":
		return starlark.MakeInt(int(v.d.Hours())), nil
	case "minutes":
		return starlark.MakeInt(int(v.d.Minutes())), nil
	case "seconds":
		return starlark.MakeInt(int(v.d.Seconds())), nil
	case "total_seconds":
		return starlark.NewBuiltin("total_seconds", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			if err := starlark.UnpackPositionalArgs("total_seconds", args, kwargs, 0); err != nil {
				return nil, err
			}
			return starlark.Float(v.d.Seconds()), nil
		}), nil
	}
	return nil, nil
}
func (v *timedeltaValue) AttrNames() []string {
	return []string{"days", "hours", "minutes", "seconds", "total_seconds"}
}

func (v *timedeltaValue) Binary(op syntax.Token, y starlark.Value, side starlark.Side) (starlark.Value, error) {
	switch op {
	case syntax.PLUS:
		if oy, ok := y.(*timedeltaValue); ok {
			if side == starlark.Left {
				return &timedeltaValue{v.d + oy.d}, nil
			}
			return &timedeltaValue{oy.d + v.d}, nil
		}
	case syntax.MINUS:
		if oy, ok := y.(*timedeltaValue); ok {
			if side == starlark.Left {
				return &timedeltaValue{v.d - oy.d}, nil
			}
			return &timedeltaValue{oy.d - v.d}, nil
		}
	}
	return nil, nil
}

func (v *timedeltaValue) CompareSameType(op syntax.Token, yv starlark.Value, _ int) (bool, error) {
	y := yv.(*timedeltaValue)
	switch op {
	case syntax.EQL:
		return v.d == y.d, nil
	case syntax.NEQ:
		return v.d != y.d, nil
	case syntax.LT:
		return v.d < y.d, nil
	case syntax.LE:
		return v.d <= y.d, nil
	case syntax.GT:
		return v.d > y.d, nil
	case syntax.GE:
		return v.d >= y.d, nil
	}
	return false, fmt.Errorf("timedelta: unsupported comparison %s", op)
}

// timedeltaType is the callable constructor: timedelta(days=0, hours=0, minutes=0, seconds=0)
type timedeltaType struct{}

var _ starlark.Callable = timedeltaType{}

func (timedeltaType) String() string        { return "<type 'timedelta'>" }
func (timedeltaType) Type() string          { return "builtin_type" }
func (timedeltaType) Freeze()               {}
func (timedeltaType) Truth() starlark.Bool  { return true }
func (timedeltaType) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: builtin_type") }
func (timedeltaType) Name() string          { return "timedelta" }

func (timedeltaType) CallInternal(_ *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var days, hours, minutes, seconds starlark.Value
	if err := starlark.UnpackArgs("timedelta", args, kwargs,
		"days?", &days,
		"hours?", &hours,
		"minutes?", &minutes,
		"seconds?", &seconds,
	); err != nil {
		return nil, err
	}
	var d time.Duration
	if days != nil {
		n, err := toInt64(days, "timedelta", "days")
		if err != nil {
			return nil, err
		}
		d += time.Duration(n) * 24 * time.Hour
	}
	if hours != nil {
		n, err := toInt64(hours, "timedelta", "hours")
		if err != nil {
			return nil, err
		}
		d += time.Duration(n) * time.Hour
	}
	if minutes != nil {
		n, err := toInt64(minutes, "timedelta", "minutes")
		if err != nil {
			return nil, err
		}
		d += time.Duration(n) * time.Minute
	}
	if seconds != nil {
		n, err := toInt64(seconds, "timedelta", "seconds")
		if err != nil {
			return nil, err
		}
		d += time.Duration(n) * time.Second
	}
	return &timedeltaValue{d}, nil
}

// ─── date ─────────────────────────────────────────────────────────────────────

// dateValue represents a calendar date (no time component).
type dateValue struct {
	t time.Time // time is always midnight UTC
}

var _ starlark.Value = (*dateValue)(nil)
var _ starlark.HasAttrs = (*dateValue)(nil)
var _ starlark.HasBinary = (*dateValue)(nil)
var _ starlark.Comparable = (*dateValue)(nil)

func newDate(year, month, day int) *dateValue {
	return &dateValue{time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)}
}

func (v *dateValue) String() string {
	return fmt.Sprintf("date(%d, %d, %d)", v.t.Year(), int(v.t.Month()), v.t.Day())
}
func (v *dateValue) Type() string          { return "date" }
func (v *dateValue) Freeze()               {}
func (v *dateValue) Truth() starlark.Bool  { return true }
func (v *dateValue) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: date") }

func (v *dateValue) Attr(name string) (starlark.Value, error) {
	switch name {
	case "year":
		return starlark.MakeInt(v.t.Year()), nil
	case "month":
		return starlark.MakeInt(int(v.t.Month())), nil
	case "day":
		return starlark.MakeInt(v.t.Day()), nil
	case "weekday":
		// Returns 0=Monday … 6=Sunday (Python convention)
		return starlark.NewBuiltin("weekday", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			if err := starlark.UnpackPositionalArgs("weekday", args, kwargs, 0); err != nil {
				return nil, err
			}
			wd := int(v.t.Weekday()+6) % 7 // Go Sunday=0 → Python Monday=0
			return starlark.MakeInt(wd), nil
		}), nil
	case "isoformat":
		return starlark.NewBuiltin("isoformat", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			if err := starlark.UnpackPositionalArgs("isoformat", args, kwargs, 0); err != nil {
				return nil, err
			}
			return starlark.String(v.t.Format("2006-01-02")), nil
		}), nil
	case "strftime":
		return starlark.NewBuiltin("strftime", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var layout string
			if err := starlark.UnpackPositionalArgs("strftime", args, kwargs, 1, &layout); err != nil {
				return nil, err
			}
			return starlark.String(v.t.Format(layout)), nil
		}), nil
	}
	return nil, nil
}

func (v *dateValue) AttrNames() []string {
	return []string{"year", "month", "day", "weekday", "isoformat", "strftime"}
}

func (v *dateValue) Binary(op syntax.Token, y starlark.Value, side starlark.Side) (starlark.Value, error) {
	switch op {
	case syntax.PLUS:
		if td, ok := y.(*timedeltaValue); ok {
			t := v.t.Add(td.d)
			return newDate(t.Year(), int(t.Month()), t.Day()), nil
		}
	case syntax.MINUS:
		if side == starlark.Left {
			if td, ok := y.(*timedeltaValue); ok {
				t := v.t.Add(-td.d)
				return newDate(t.Year(), int(t.Month()), t.Day()), nil
			}
			if od, ok := y.(*dateValue); ok {
				diff := v.t.Sub(od.t)
				return &timedeltaValue{diff}, nil
			}
		}
	}
	return nil, nil
}

func (v *dateValue) CompareSameType(op syntax.Token, yv starlark.Value, _ int) (bool, error) {
	y := yv.(*dateValue)
	switch op {
	case syntax.EQL:
		return v.t.Equal(y.t), nil
	case syntax.NEQ:
		return !v.t.Equal(y.t), nil
	case syntax.LT:
		return v.t.Before(y.t), nil
	case syntax.LE:
		return !v.t.After(y.t), nil
	case syntax.GT:
		return v.t.After(y.t), nil
	case syntax.GE:
		return !v.t.Before(y.t), nil
	}
	return false, fmt.Errorf("date: unsupported comparison %s", op)
}

// dateType is the callable constructor for date + class methods (today, fromisoformat).
type dateType struct{}

var _ starlark.Callable = dateType{}
var _ starlark.HasAttrs = dateType{}

func (dateType) String() string        { return "<type 'date'>" }
func (dateType) Type() string          { return "builtin_type" }
func (dateType) Freeze()               {}
func (dateType) Truth() starlark.Bool  { return true }
func (dateType) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: builtin_type") }
func (dateType) Name() string          { return "date" }

func (dateType) CallInternal(_ *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var yearV, monthV, dayV starlark.Value
	if err := starlark.UnpackPositionalArgs("date", args, kwargs, 3, &yearV, &monthV, &dayV); err != nil {
		return nil, err
	}
	year, err := toInt(yearV, "date", "year")
	if err != nil {
		return nil, err
	}
	month, err := toInt(monthV, "date", "month")
	if err != nil {
		return nil, err
	}
	day, err := toInt(dayV, "date", "day")
	if err != nil {
		return nil, err
	}
	return newDate(year, month, day), nil
}

func (dateType) Attr(name string) (starlark.Value, error) {
	switch name {
	case "today":
		return starlark.NewBuiltin("today", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			if err := starlark.UnpackPositionalArgs("today", args, kwargs, 0); err != nil {
				return nil, err
			}
			t := time.Now().UTC()
			return newDate(t.Year(), int(t.Month()), t.Day()), nil
		}), nil
	case "fromisoformat":
		return starlark.NewBuiltin("fromisoformat", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var s string
			if err := starlark.UnpackPositionalArgs("fromisoformat", args, kwargs, 1, &s); err != nil {
				return nil, err
			}
			t, err := time.Parse("2006-01-02", s)
			if err != nil {
				return nil, fmt.Errorf("date.fromisoformat: invalid date %q, expected YYYY-MM-DD", s)
			}
			return newDate(t.Year(), int(t.Month()), t.Day()), nil
		}), nil
	}
	return nil, nil
}

func (dateType) AttrNames() []string { return []string{"today", "fromisoformat"} }

// ─── datetime ─────────────────────────────────────────────────────────────────

// dateTimeValue represents a date and time.
type dateTimeValue struct {
	t time.Time
}

var _ starlark.Value = (*dateTimeValue)(nil)
var _ starlark.HasAttrs = (*dateTimeValue)(nil)
var _ starlark.HasBinary = (*dateTimeValue)(nil)
var _ starlark.Comparable = (*dateTimeValue)(nil)

func newDateTime(t time.Time) *dateTimeValue {
	return &dateTimeValue{t.UTC()}
}

func (v *dateTimeValue) String() string {
	return fmt.Sprintf("datetime(%d, %d, %d, %d, %d, %d)",
		v.t.Year(), int(v.t.Month()), v.t.Day(),
		v.t.Hour(), v.t.Minute(), v.t.Second())
}
func (v *dateTimeValue) Type() string          { return "datetime" }
func (v *dateTimeValue) Freeze()               {}
func (v *dateTimeValue) Truth() starlark.Bool  { return true }
func (v *dateTimeValue) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: datetime") }

func (v *dateTimeValue) Attr(name string) (starlark.Value, error) {
	switch name {
	case "year":
		return starlark.MakeInt(v.t.Year()), nil
	case "month":
		return starlark.MakeInt(int(v.t.Month())), nil
	case "day":
		return starlark.MakeInt(v.t.Day()), nil
	case "hour":
		return starlark.MakeInt(v.t.Hour()), nil
	case "minute":
		return starlark.MakeInt(v.t.Minute()), nil
	case "second":
		return starlark.MakeInt(v.t.Second()), nil
	case "weekday":
		return starlark.NewBuiltin("weekday", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			if err := starlark.UnpackPositionalArgs("weekday", args, kwargs, 0); err != nil {
				return nil, err
			}
			wd := int(v.t.Weekday()+6) % 7
			return starlark.MakeInt(wd), nil
		}), nil
	case "date":
		return starlark.NewBuiltin("date", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			if err := starlark.UnpackPositionalArgs("date", args, kwargs, 0); err != nil {
				return nil, err
			}
			return newDate(v.t.Year(), int(v.t.Month()), v.t.Day()), nil
		}), nil
	case "isoformat":
		return starlark.NewBuiltin("isoformat", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			if err := starlark.UnpackPositionalArgs("isoformat", args, kwargs, 0); err != nil {
				return nil, err
			}
			return starlark.String(v.t.Format(time.RFC3339)), nil
		}), nil
	case "strftime":
		return starlark.NewBuiltin("strftime", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var layout string
			if err := starlark.UnpackPositionalArgs("strftime", args, kwargs, 1, &layout); err != nil {
				return nil, err
			}
			return starlark.String(v.t.Format(layout)), nil
		}), nil
	case "timestamp":
		return starlark.NewBuiltin("timestamp", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			if err := starlark.UnpackPositionalArgs("timestamp", args, kwargs, 0); err != nil {
				return nil, err
			}
			return starlark.MakeInt64(v.t.Unix()), nil
		}), nil
	}
	return nil, nil
}

func (v *dateTimeValue) AttrNames() []string {
	return []string{"year", "month", "day", "hour", "minute", "second", "weekday", "date", "isoformat", "strftime", "timestamp"}
}

func (v *dateTimeValue) Binary(op syntax.Token, y starlark.Value, side starlark.Side) (starlark.Value, error) {
	switch op {
	case syntax.PLUS:
		if td, ok := y.(*timedeltaValue); ok {
			return newDateTime(v.t.Add(td.d)), nil
		}
	case syntax.MINUS:
		if side == starlark.Left {
			if td, ok := y.(*timedeltaValue); ok {
				return newDateTime(v.t.Add(-td.d)), nil
			}
			if od, ok := y.(*dateTimeValue); ok {
				diff := v.t.Sub(od.t)
				return &timedeltaValue{diff}, nil
			}
		}
	}
	return nil, nil
}

func (v *dateTimeValue) CompareSameType(op syntax.Token, yv starlark.Value, _ int) (bool, error) {
	y := yv.(*dateTimeValue)
	switch op {
	case syntax.EQL:
		return v.t.Equal(y.t), nil
	case syntax.NEQ:
		return !v.t.Equal(y.t), nil
	case syntax.LT:
		return v.t.Before(y.t), nil
	case syntax.LE:
		return !v.t.After(y.t), nil
	case syntax.GT:
		return v.t.After(y.t), nil
	case syntax.GE:
		return !v.t.Before(y.t), nil
	}
	return false, fmt.Errorf("datetime: unsupported comparison %s", op)
}

// datetimeType is the callable constructor for datetime + class methods.
type datetimeType struct{}

var _ starlark.Callable = datetimeType{}
var _ starlark.HasAttrs = datetimeType{}

func (datetimeType) String() string        { return "<type 'datetime'>" }
func (datetimeType) Type() string          { return "builtin_type" }
func (datetimeType) Freeze()               {}
func (datetimeType) Truth() starlark.Bool  { return true }
func (datetimeType) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: builtin_type") }
func (datetimeType) Name() string          { return "datetime" }

func (datetimeType) CallInternal(_ *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var yearV, monthV, dayV starlark.Value
	var hourV, minuteV, secondV starlark.Value
	if err := starlark.UnpackArgs("datetime", args, kwargs,
		"year", &yearV, "month", &monthV, "day", &dayV,
		"hour?", &hourV, "minute?", &minuteV, "second?", &secondV,
	); err != nil {
		return nil, err
	}
	year, err := toInt(yearV, "datetime", "year")
	if err != nil {
		return nil, err
	}
	month, err := toInt(monthV, "datetime", "month")
	if err != nil {
		return nil, err
	}
	day, err := toInt(dayV, "datetime", "day")
	if err != nil {
		return nil, err
	}
	hour := 0
	if hourV != nil {
		hour, err = toInt(hourV, "datetime", "hour")
		if err != nil {
			return nil, err
		}
	}
	minute := 0
	if minuteV != nil {
		minute, err = toInt(minuteV, "datetime", "minute")
		if err != nil {
			return nil, err
		}
	}
	second := 0
	if secondV != nil {
		second, err = toInt(secondV, "datetime", "second")
		if err != nil {
			return nil, err
		}
	}
	t := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC)
	return newDateTime(t), nil
}

func (datetimeType) Attr(name string) (starlark.Value, error) {
	switch name {
	case "now", "utcnow":
		return starlark.NewBuiltin(name, func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			if err := starlark.UnpackPositionalArgs(name, args, kwargs, 0); err != nil {
				return nil, err
			}
			return newDateTime(time.Now()), nil
		}), nil
	case "fromisoformat":
		return starlark.NewBuiltin("fromisoformat", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var s string
			if err := starlark.UnpackPositionalArgs("fromisoformat", args, kwargs, 1, &s); err != nil {
				return nil, err
			}
			layouts := []string{
				time.RFC3339,
				"2006-01-02T15:04:05",
				"2006-01-02 15:04:05",
				"2006-01-02",
			}
			var parsed time.Time
			var parseErr error
			for _, layout := range layouts {
				parsed, parseErr = time.Parse(layout, s)
				if parseErr == nil {
					break
				}
			}
			if parseErr != nil {
				return nil, fmt.Errorf("datetime.fromisoformat: cannot parse %q", s)
			}
			return newDateTime(parsed), nil
		}), nil
	case "fromtimestamp":
		return starlark.NewBuiltin("fromtimestamp", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var tsV starlark.Value
			if err := starlark.UnpackPositionalArgs("fromtimestamp", args, kwargs, 1, &tsV); err != nil {
				return nil, err
			}
			ts, err := toInt64(tsV, "datetime.fromtimestamp", "ts")
			if err != nil {
				return nil, err
			}
			return newDateTime(time.Unix(ts, 0)), nil
		}), nil
	}
	return nil, nil
}

func (datetimeType) AttrNames() []string {
	return []string{"now", "utcnow", "fromisoformat", "fromtimestamp"}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func toInt64(v starlark.Value, fn, param string) (int64, error) {
	switch x := v.(type) {
	case starlark.Int:
		n, ok := x.Int64()
		if !ok {
			return 0, fmt.Errorf("%s: %s value out of range", fn, param)
		}
		return n, nil
	case starlark.Float:
		return int64(x), nil
	}
	return 0, fmt.Errorf("%s: %s must be int, got %s", fn, param, v.Type())
}

func toInt(v starlark.Value, fn, param string) (int, error) {
	n, err := toInt64(v, fn, param)
	return int(n), err
}

// isoFormatLayouts tries a set of common ISO 8601 layouts when parsing.
func isoFormatLayouts() []string {
	return []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
}

// parseISODate parses a string as a date using common layouts.
// Returns (time.Time, true) on success.
func parseISODate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range isoFormatLayouts() {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}
