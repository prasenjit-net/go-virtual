package store

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/prasenjit/go-virtual/internal/models"
)

// StoreBuiltin implements starlark.Value and starlark.HasAttrs.
// It exposes the session store to Starlark scripts via:
//
//	store.get("key")           → value or None
//	store.get("key", default)  → value or default
//	store.set("key", value)    → None
//	store.has("key")           → bool
//	store.delete("key")        → None
//	store.keys()               → list of strings
//	store.collection("name")   → CollectionBuiltin
type StoreBuiltin struct {
	session         SessionState
	collectionStore CollectionBackend
	accessLog       *[]models.StoreAccessEvent
}

// NewStoreBuiltin wraps a session for Starlark access.
// collectionStore backs store.collection("name") calls.
// accessLog is appended to for each operation (used for trace recording).
func NewStoreBuiltin(sess SessionState, collectionStore CollectionBackend, accessLog *[]models.StoreAccessEvent) *StoreBuiltin {
	return &StoreBuiltin{session: sess, collectionStore: collectionStore, accessLog: accessLog}
}

// Starlark interface implementation ─────────────────────────────────────────

func (sb *StoreBuiltin) String() string        { return "store" }
func (sb *StoreBuiltin) Type() string          { return "store" }
func (sb *StoreBuiltin) Freeze()               {}
func (sb *StoreBuiltin) Truth() starlark.Bool  { return starlark.True }
func (sb *StoreBuiltin) Hash() (uint32, error) { return 0, fmt.Errorf("store is not hashable") }

// Attr returns the named method.
func (sb *StoreBuiltin) Attr(name string) (starlark.Value, error) {
	switch name {
	case "get":
		return starlark.NewBuiltin("store.get", sb.builtinGet), nil
	case "set":
		return starlark.NewBuiltin("store.set", sb.builtinSet), nil
	case "has":
		return starlark.NewBuiltin("store.has", sb.builtinHas), nil
	case "delete":
		return starlark.NewBuiltin("store.delete", sb.builtinDelete), nil
	case "keys":
		return starlark.NewBuiltin("store.keys", sb.builtinKeys), nil
	case "collection":
		return starlark.NewBuiltin("store.collection", sb.builtinCollection), nil
	}
	return nil, nil
}

// AttrNames returns all available method names (for dir()).
func (sb *StoreBuiltin) AttrNames() []string {
	return []string{"get", "set", "has", "delete", "keys", "collection"}
}

// ── Method implementations ──────────────────────────────────────────────────

// store.get("key") or store.get("key", default)
func (sb *StoreBuiltin) builtinGet(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key string
	var defaultVal starlark.Value = starlark.None

	if err := starlark.UnpackPositionalArgs("store.get", args, kwargs, 1, &key, &defaultVal); err != nil {
		return nil, err
	}

	val, ok := sb.session.Get(key)

	var result starlark.Value
	if ok {
		result = goToStar(val)
	} else {
		result = defaultVal
	}

	if sb.accessLog != nil {
		*sb.accessLog = append(*sb.accessLog, models.StoreAccessEvent{Op: "get", Key: key, Value: val})
	}

	return result, nil
}

// store.set("key", value)
func (sb *StoreBuiltin) builtinSet(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key string
	var val starlark.Value

	if err := starlark.UnpackPositionalArgs("store.set", args, kwargs, 2, &key, &val); err != nil {
		return nil, err
	}

	goVal := starToGo(val)
	if err := sb.session.Set(key, goVal); err != nil {
		return nil, err
	}

	if sb.accessLog != nil {
		*sb.accessLog = append(*sb.accessLog, models.StoreAccessEvent{Op: "set", Key: key, Value: goVal})
	}

	return starlark.None, nil
}

// store.has("key") → bool
func (sb *StoreBuiltin) builtinHas(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key string
	if err := starlark.UnpackPositionalArgs("store.has", args, kwargs, 1, &key); err != nil {
		return nil, err
	}

	ok := sb.session.Has(key)

	if sb.accessLog != nil {
		*sb.accessLog = append(*sb.accessLog, models.StoreAccessEvent{Op: "has", Key: key})
	}

	return starlark.Bool(ok), nil
}

// store.delete("key")
func (sb *StoreBuiltin) builtinDelete(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key string
	if err := starlark.UnpackPositionalArgs("store.delete", args, kwargs, 1, &key); err != nil {
		return nil, err
	}

	if err := sb.session.Delete(key); err != nil {
		return nil, err
	}

	if sb.accessLog != nil {
		*sb.accessLog = append(*sb.accessLog, models.StoreAccessEvent{Op: "delete", Key: key})
	}

	return starlark.None, nil
}

// store.keys() → list[str]
func (sb *StoreBuiltin) builtinKeys(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs("store.keys", args, kwargs, 0); err != nil {
		return nil, err
	}

	keys := sb.session.Keys()
	elems := make([]starlark.Value, len(keys))
	for i, k := range keys {
		elems[i] = starlark.String(k)
	}

	if sb.accessLog != nil {
		*sb.accessLog = append(*sb.accessLog, models.StoreAccessEvent{Op: "keys"})
	}

	return starlark.NewList(elems), nil
}

// store.collection("name") → CollectionBuiltin
func (sb *StoreBuiltin) builtinCollection(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	if err := starlark.UnpackPositionalArgs("store.collection", args, kwargs, 1, &name); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("store.collection: name must not be empty")
	}
	return newCollectionBuiltin(name, sb.collectionStore, sb.session, sb.accessLog), nil
}

func goToStar(v any) starlark.Value {
	if v == nil {
		return starlark.None
	}
	switch t := v.(type) {
	case bool:
		return starlark.Bool(t)
	case int:
		return starlark.MakeInt(t)
	case int64:
		return starlark.MakeInt64(t)
	case float64:
		return starlark.Float(t)
	case string:
		return starlark.String(t)
	case map[string]any:
		d := new(starlark.Dict)
		for k, val := range t {
			_ = d.SetKey(starlark.String(k), goToStar(val))
		}
		return d
	case []any:
		elems := make([]starlark.Value, len(t))
		for i, elem := range t {
			elems[i] = goToStar(elem)
		}
		return starlark.NewList(elems)
	}
	return starlark.None
}

func starToGo(v starlark.Value) any {
	if v == nil || v == starlark.None {
		return nil
	}
	switch t := v.(type) {
	case starlark.Bool:
		return bool(t)
	case starlark.Int:
		n, _ := t.Int64()
		return n
	case starlark.Float:
		return float64(t)
	case starlark.String:
		return string(t)
	case *starlark.Dict:
		m := make(map[string]any, t.Len())
		for _, kv := range t.Items() {
			if ks, ok := kv[0].(starlark.String); ok {
				m[string(ks)] = starToGo(kv[1])
			}
		}
		return m
	case *starlark.List:
		elems := make([]any, t.Len())
		for i := range elems {
			elems[i] = starToGo(t.Index(i))
		}
		return elems
	}
	return nil
}
