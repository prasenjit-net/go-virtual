package store_test

import (
	"testing"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/store"
	"go.starlark.net/starlark"
)

func makeCollection(t *testing.T, name string) (starlark.Value, starlark.HasAttrs, store.SessionState, *[]models.StoreAccessEvent) {
	t.Helper()
	sess := store.NewEphemeralSession(nil)
	var accessLog []models.StoreAccessEvent
	sb := store.NewStoreBuiltin(sess, store.NewMemoryCollectionBackend(), &accessLog)
	attr, err := sb.Attr("collection")
	if err != nil || attr == nil {
		t.Fatalf("Attr(collection): %v / %v", attr, err)
	}
	val, err := starlark.Call(&starlark.Thread{Name: "collection-test"}, attr.(*starlark.Builtin), starlark.Tuple{starlark.String(name)}, nil)
	if err != nil {
		t.Fatalf("store.collection(%q): %v", name, err)
	}
	hasAttrs, ok := val.(starlark.HasAttrs)
	if !ok {
		t.Fatalf("expected collection value with attrs, got %T", val)
	}
	return val, hasAttrs, sess, &accessLog
}

func callCollectionMethod(t *testing.T, obj starlark.HasAttrs, name string, args ...starlark.Value) starlark.Value {
	t.Helper()
	attr, err := obj.Attr(name)
	if err != nil || attr == nil {
		t.Fatalf("Attr(%q): %v / %v", name, attr, err)
	}
	val, err := starlark.Call(&starlark.Thread{Name: "collection-method"}, attr.(*starlark.Builtin), starlark.Tuple(args), nil)
	if err != nil {
		t.Fatalf("Call %s: %v", name, err)
	}
	return val
}

func callCollectionMethodErr(t *testing.T, obj starlark.HasAttrs, name string, args ...starlark.Value) error {
	t.Helper()
	attr, err := obj.Attr(name)
	if err != nil || attr == nil {
		t.Fatalf("Attr(%q): %v / %v", name, attr, err)
	}
	_, err = starlark.Call(&starlark.Thread{Name: "collection-method-err"}, attr.(*starlark.Builtin), starlark.Tuple(args), nil)
	return err
}

func dict(items ...any) *starlark.Dict {
	d := new(starlark.Dict)
	for i := 0; i < len(items); i += 2 {
		_ = d.SetKey(starlark.String(items[i].(string)), items[i+1].(starlark.Value))
	}
	return d
}

func intValue(t *testing.T, v starlark.Value) int64 {
	t.Helper()
	n, ok := v.(starlark.Int)
	if !ok {
		t.Fatalf("expected starlark.Int, got %T", v)
	}
	out, ok := n.Int64()
	if !ok {
		t.Fatalf("expected int64 conversion for %v", v)
	}
	return out
}

func TestCollectionBuiltin_CRUDAndFilters(t *testing.T) {
	val, col, sess, log := makeCollection(t, "users")
	if val.String() != `collection("users")` {
		t.Fatalf("unexpected String(): %s", val.String())
	}
	if val.Type() != "collection" {
		t.Fatalf("unexpected Type(): %s", val.Type())
	}
	if val.Truth() != starlark.True {
		t.Fatal("expected collection to be truthy")
	}
	if _, err := val.Hash(); err == nil {
		t.Fatal("expected collection to be unhashable")
	}
	if len(col.AttrNames()) != 7 {
		t.Fatalf("unexpected AttrNames: %v", col.AttrNames())
	}

	callCollectionMethod(t, col, "insert", dict("name", starlark.String("alice"), "age", starlark.MakeInt(30), "active", starlark.True))
	callCollectionMethod(t, col, "insert", dict("name", starlark.String("bob"), "age", starlark.MakeInt(25), "active", starlark.False))

	all := callCollectionMethod(t, col, "findAll").(*starlark.List)
	if all.Len() != 2 {
		t.Fatalf("expected 2 docs, got %d", all.Len())
	}

	filtered := callCollectionMethod(t, col, "findAll", dict("name", starlark.String("alice"))).(*starlark.List)
	if filtered.Len() != 1 {
		t.Fatalf("expected 1 filtered doc, got %d", filtered.Len())
	}

	one := callCollectionMethod(t, col, "findOne", dict("name", starlark.String("bob")))
	bob, ok := one.(*starlark.Dict)
	if !ok {
		t.Fatalf("expected dict, got %T", one)
	}
	nameVal, _, _ := bob.Get(starlark.String("name"))
	if nameVal.(starlark.String) != "bob" {
		t.Fatalf("expected bob doc, got %v", nameVal)
	}

	if got := intValue(t, callCollectionMethod(t, col, "count")); got != 2 {
		t.Fatalf("expected count 2, got %d", got)
	}
	if got := intValue(t, callCollectionMethod(t, col, "count", dict("name", starlark.String("alice")))); got != 1 {
		t.Fatalf("expected filtered count 1, got %d", got)
	}

	if got := intValue(t, callCollectionMethod(t, col, "update", dict("name", starlark.String("alice")), dict("age", starlark.MakeInt(31), "city", starlark.String("Paris")))); got != 1 {
		t.Fatalf("expected 1 updated doc, got %d", got)
	}
	alice := callCollectionMethod(t, col, "findOne", dict("name", starlark.String("alice"))).(*starlark.Dict)
	ageVal, _, _ := alice.Get(starlark.String("age"))
	cityVal, _, _ := alice.Get(starlark.String("city"))
	if intValue(t, ageVal) != 31 || cityVal.(starlark.String) != "Paris" {
		t.Fatalf("unexpected updated alice doc: %v %v", ageVal, cityVal)
	}

	if got := intValue(t, callCollectionMethod(t, col, "remove", dict("age", starlark.MakeInt(25)))); got != 1 {
		t.Fatalf("expected 1 removed doc, got %d", got)
	}
	if got := intValue(t, callCollectionMethod(t, col, "count")); got != 1 {
		t.Fatalf("expected count 1 after remove, got %d", got)
	}

	callCollectionMethod(t, col, "clear")
	if got := intValue(t, callCollectionMethod(t, col, "count")); got != 0 {
		t.Fatalf("expected empty collection after clear, got %d", got)
	}

	// After clear, session must have at least one event under __cevt__users
	raw, ok := sess.Get(store.CollectionEventKeyPrefix + "users")
	if !ok {
		t.Fatal("expected event log key in session after mutations")
	}
	events := store.LoadEvents(sess, "users")
	hasClear := false
	for _, ev := range events {
		if ev.Op == "clear" {
			hasClear = true
		}
	}
	if !hasClear {
		t.Fatalf("expected a clear event in session log, got %#v", raw)
	}

	seenOps := map[string]bool{}
	for _, entry := range *log {
		seenOps[entry.Op] = true
	}
	for _, op := range []string{"collection.insert", "collection.findAll", "collection.findOne", "collection.count", "collection.update", "collection.remove", "collection.clear"} {
		if !seenOps[op] {
			t.Fatalf("missing access log op %q in %#v", op, *log)
		}
	}
}

func TestCollectionBuiltin_ErrorPaths(t *testing.T) {
	_, col, _, _ := makeCollection(t, "users")

	if err := callCollectionMethodErr(t, col, "insert", starlark.String("nope")); err == nil {
		t.Fatal("expected insert to reject non-dict input")
	}
	if err := callCollectionMethodErr(t, col, "findAll", starlark.String("nope")); err == nil {
		t.Fatal("expected findAll to reject non-dict filter")
	}
	if err := callCollectionMethodErr(t, col, "findOne", starlark.String("nope")); err == nil {
		t.Fatal("expected findOne to reject non-dict filter")
	}
	if err := callCollectionMethodErr(t, col, "update", dict("name", starlark.String("alice")), starlark.String("nope")); err == nil {
		t.Fatal("expected update to reject non-dict changes")
	}
	if err := callCollectionMethodErr(t, col, "remove", starlark.String("nope")); err == nil {
		t.Fatal("expected remove to reject non-dict filter")
	}
	if err := callCollectionMethodErr(t, col, "count", starlark.String("nope")); err == nil {
		t.Fatal("expected count to reject non-dict filter")
	}

	sb := store.NewStoreBuiltin(store.NewEphemeralSession(nil), nil, nil)
	attr, err := sb.Attr("collection")
	if err != nil || attr == nil {
		t.Fatalf("Attr(collection): %v / %v", attr, err)
	}
	if _, err := starlark.Call(&starlark.Thread{Name: "collection-empty"}, attr.(*starlark.Builtin), starlark.Tuple{starlark.String("")}, nil); err == nil {
		t.Fatal("expected empty collection name to fail")
	}
}
