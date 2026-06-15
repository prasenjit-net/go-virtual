package store

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/prasenjit/go-virtual/internal/models"
)

// CollectionBuiltin is the Starlark object returned by store.collection("name").
// Reads merge the global CollectionBackend base with the session's event log.
// Writes append events to the session log without touching the global base.
//
//	col.insert(doc)                → None
//	col.findAll()                  → list
//	col.findAll(filter)            → filtered list
//	col.findOne(filter)            → doc or None
//	col.update(filter, changes)    → int (matched count)
//	col.remove(filter)             → int (removed count)
//	col.count()                    → int
//	col.count(filter)              → int (filtered count)
//	col.clear()                    → None
type CollectionBuiltin struct {
	name      string
	backend   CollectionBackend
	session   SessionState
	accessLog *[]models.StoreAccessEvent
}

func newCollectionBuiltin(name string, backend CollectionBackend, sess SessionState, log *[]models.StoreAccessEvent) *CollectionBuiltin {
	return &CollectionBuiltin{name: name, backend: backend, session: sess, accessLog: log}
}

// ── Starlark value interface ─────────────────────────────────────────────────

func (cb *CollectionBuiltin) String() string        { return fmt.Sprintf("collection(%q)", cb.name) }
func (cb *CollectionBuiltin) Type() string          { return "collection" }
func (cb *CollectionBuiltin) Freeze()               {}
func (cb *CollectionBuiltin) Truth() starlark.Bool  { return starlark.True }
func (cb *CollectionBuiltin) Hash() (uint32, error) { return 0, fmt.Errorf("collection is not hashable") }

func (cb *CollectionBuiltin) Attr(name string) (starlark.Value, error) {
	switch name {
	case "insert":
		return starlark.NewBuiltin("collection.insert", cb.builtinInsert), nil
	case "findAll":
		return starlark.NewBuiltin("collection.findAll", cb.builtinFindAll), nil
	case "findOne":
		return starlark.NewBuiltin("collection.findOne", cb.builtinFindOne), nil
	case "update":
		return starlark.NewBuiltin("collection.update", cb.builtinUpdate), nil
	case "remove":
		return starlark.NewBuiltin("collection.remove", cb.builtinRemove), nil
	case "count":
		return starlark.NewBuiltin("collection.count", cb.builtinCount), nil
	case "clear":
		return starlark.NewBuiltin("collection.clear", cb.builtinClear), nil
	}
	return nil, nil
}

func (cb *CollectionBuiltin) AttrNames() []string {
	return []string{"insert", "findAll", "findOne", "update", "remove", "count", "clear"}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (cb *CollectionBuiltin) load() ([]map[string]any, error) {
	base, err := cb.backend.GetAll(cb.name)
	if err != nil {
		return nil, err
	}
	return ReplayEvents(base, LoadEvents(cb.session, cb.name)), nil
}

func (cb *CollectionBuiltin) appendEvent(evt CollectionEvent) error {
	return AppendEvent(cb.session, cb.name, evt)
}

func (cb *CollectionBuiltin) logOp(op string) {
	if cb.accessLog != nil {
		*cb.accessLog = append(*cb.accessLog, models.StoreAccessEvent{Op: "collection." + op, Key: cb.name})
	}
}

// ── method implementations ───────────────────────────────────────────────────

// col.insert(doc) → None
func (cb *CollectionBuiltin) builtinInsert(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var docVal starlark.Value
	if err := starlark.UnpackPositionalArgs("collection.insert", args, kwargs, 1, &docVal); err != nil {
		return nil, err
	}
	doc, err := starToDoc(docVal)
	if err != nil {
		return nil, fmt.Errorf("collection.insert: %w", err)
	}
	if err := cb.appendEvent(CollectionEvent{Op: "insert", Data: doc}); err != nil {
		return nil, err
	}
	cb.logOp("insert")
	return starlark.None, nil
}

// col.findAll() or col.findAll(filter) → list
func (cb *CollectionBuiltin) builtinFindAll(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var filterVal starlark.Value = starlark.None
	if err := starlark.UnpackPositionalArgs("collection.findAll", args, kwargs, 0, &filterVal); err != nil {
		return nil, err
	}
	cb.logOp("findAll")

	docs, err := cb.load()
	if err != nil {
		return nil, err
	}
	if filterVal == starlark.None || filterVal == nil {
		return docsToStar(docs), nil
	}
	filter, err := starToDoc(filterVal)
	if err != nil {
		return nil, fmt.Errorf("collection.findAll filter: %w", err)
	}
	var out []map[string]any
	for _, doc := range docs {
		if matchesBuiltinFilter(doc, filter) {
			out = append(out, doc)
		}
	}
	return docsToStar(out), nil
}

// col.findOne(filter) → dict or None
func (cb *CollectionBuiltin) builtinFindOne(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var filterVal starlark.Value
	if err := starlark.UnpackPositionalArgs("collection.findOne", args, kwargs, 1, &filterVal); err != nil {
		return nil, err
	}
	filter, err := starToDoc(filterVal)
	if err != nil {
		return nil, fmt.Errorf("collection.findOne filter: %w", err)
	}
	cb.logOp("findOne")

	docs, err := cb.load()
	if err != nil {
		return nil, err
	}
	for _, doc := range docs {
		if matchesBuiltinFilter(doc, filter) {
			d := new(starlark.Dict)
			for k, v := range doc {
				_ = d.SetKey(starlark.String(k), goToStar(v))
			}
			return d, nil
		}
	}
	return starlark.None, nil
}

// col.update(filter, changes) → int (number of docs updated)
func (cb *CollectionBuiltin) builtinUpdate(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var filterVal, changesVal starlark.Value
	if err := starlark.UnpackPositionalArgs("collection.update", args, kwargs, 2, &filterVal, &changesVal); err != nil {
		return nil, err
	}
	filter, err := starToDoc(filterVal)
	if err != nil {
		return nil, fmt.Errorf("collection.update filter: %w", err)
	}
	changes, err := starToDoc(changesVal)
	if err != nil {
		return nil, fmt.Errorf("collection.update changes: %w", err)
	}
	cb.logOp("update")

	docs, err := cb.load()
	if err != nil {
		return nil, err
	}
	count := 0
	for _, doc := range docs {
		if matchesBuiltinFilter(doc, filter) {
			count++
		}
	}
	if count > 0 {
		if err := cb.appendEvent(CollectionEvent{Op: "update", Filter: filter, Data: changes}); err != nil {
			return nil, err
		}
	}
	return starlark.MakeInt(count), nil
}

// col.remove(filter) → int (number of docs removed)
func (cb *CollectionBuiltin) builtinRemove(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var filterVal starlark.Value
	if err := starlark.UnpackPositionalArgs("collection.remove", args, kwargs, 1, &filterVal); err != nil {
		return nil, err
	}
	filter, err := starToDoc(filterVal)
	if err != nil {
		return nil, fmt.Errorf("collection.remove filter: %w", err)
	}
	cb.logOp("remove")

	docs, err := cb.load()
	if err != nil {
		return nil, err
	}
	removed := 0
	for _, doc := range docs {
		if matchesBuiltinFilter(doc, filter) {
			removed++
		}
	}
	if removed > 0 {
		if err := cb.appendEvent(CollectionEvent{Op: "delete", Filter: filter}); err != nil {
			return nil, err
		}
	}
	return starlark.MakeInt(removed), nil
}

// col.count() or col.count(filter) → int
func (cb *CollectionBuiltin) builtinCount(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var filterVal starlark.Value = starlark.None
	if err := starlark.UnpackPositionalArgs("collection.count", args, kwargs, 0, &filterVal); err != nil {
		return nil, err
	}
	cb.logOp("count")

	docs, err := cb.load()
	if err != nil {
		return nil, err
	}
	if filterVal == starlark.None || filterVal == nil {
		return starlark.MakeInt(len(docs)), nil
	}
	filter, err := starToDoc(filterVal)
	if err != nil {
		return nil, fmt.Errorf("collection.count filter: %w", err)
	}
	n := 0
	for _, doc := range docs {
		if matchesBuiltinFilter(doc, filter) {
			n++
		}
	}
	return starlark.MakeInt(n), nil
}

// col.clear() → None
func (cb *CollectionBuiltin) builtinClear(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs("collection.clear", args, kwargs, 0); err != nil {
		return nil, err
	}
	cb.logOp("clear")
	return starlark.None, cb.appendEvent(CollectionEvent{Op: "clear"})
}

// ── filter helper ─────────────────────────────────────────────────────────────

func matchesBuiltinFilter(doc, filter map[string]any) bool {
	for k, fv := range filter {
		dv, ok := doc[k]
		if !ok {
			return false
		}
		if fmt.Sprintf("%v", dv) != fmt.Sprintf("%v", fv) {
			return false
		}
	}
	return true
}

// ── Starlark ↔ Go conversion helpers ─────────────────────────────────────────

// starToDoc converts a Starlark dict to a Go map.
func starToDoc(v starlark.Value) (map[string]any, error) {
	d, ok := v.(*starlark.Dict)
	if !ok {
		return nil, fmt.Errorf("expected dict, got %s", v.Type())
	}
	m := make(map[string]any, d.Len())
	for _, item := range d.Items() {
		k, ok := item[0].(starlark.String)
		if !ok {
			return nil, fmt.Errorf("dict key must be string, got %s", item[0].Type())
		}
		m[string(k)] = starToGo(item[1])
	}
	return m, nil
}

// docsToStar converts a slice of docs to a Starlark list of dicts.
func docsToStar(docs []map[string]any) *starlark.List {
	elems := make([]starlark.Value, len(docs))
	for i, doc := range docs {
		d := new(starlark.Dict)
		for k, v := range doc {
			_ = d.SetKey(starlark.String(k), goToStar(v))
		}
		elems[i] = d
	}
	return starlark.NewList(elems)
}
