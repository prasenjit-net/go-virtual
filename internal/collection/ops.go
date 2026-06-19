package collection

import (
	"fmt"

	"github.com/prasenjit/go-virtual/internal/store"
)

// Ops performs collection operations for one named collection. Each mutating
// method appends a CollectionEvent to the session's event log and returns the
// result computed by replaying all events on top of the global base.
type Ops struct {
	backend store.CollectionBackend
	sess    store.SessionState
	name    string
}

// NewOps returns an Ops for the named collection, backed by the given
// CollectionBackend and recording events in sess.
func NewOps(name string, backend store.CollectionBackend, sess store.SessionState) *Ops {
	return &Ops{backend: backend, sess: sess, name: name}
}

// load returns the session's current view of the collection (base + events).
func (o *Ops) load() ([]map[string]any, error) {
	base, err := o.backend.GetAll(o.name)
	if err != nil {
		return nil, err
	}
	return store.ReplayEvents(base, store.LoadEvents(o.sess, o.name)), nil
}

// FindOne returns the first document matching filter, or nil if none found.
func (o *Ops) FindOne(filter map[string]any) (map[string]any, error) {
	docs, err := o.load()
	if err != nil {
		return nil, err
	}
	for _, doc := range docs {
		if matchesFilter(doc, filter) {
			return copyDoc(doc), nil
		}
	}
	return nil, nil
}

// FindMany returns all documents matching filter (empty slice if none).
func (o *Ops) FindMany(filter map[string]any) ([]map[string]any, error) {
	docs, err := o.load()
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for _, doc := range docs {
		if matchesFilter(doc, filter) {
			out = append(out, copyDoc(doc))
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, nil
}

// Insert appends a new document and returns it (with auto-assigned _id).
func (o *Ops) Insert(data map[string]any) (map[string]any, error) {
	if err := store.AppendEvent(o.sess, o.name, store.CollectionEvent{Op: "insert", Data: copyDoc(data)}); err != nil {
		return nil, err
	}
	docs, err := o.load()
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return map[string]any{}, nil
	}
	return copyDoc(docs[len(docs)-1]), nil
}

// Update finds the first document matching filter, merges changes into it, and
// returns the post-update document. Returns nil if no document matched.
func (o *Ops) Update(filter, changes map[string]any) (map[string]any, error) {
	docs, err := o.load()
	if err != nil {
		return nil, err
	}
	var matched map[string]any
	for _, doc := range docs {
		if matchesFilter(doc, filter) {
			matched = doc
			break
		}
	}
	if matched == nil {
		return nil, nil
	}
	if err := store.AppendEvent(o.sess, o.name, store.CollectionEvent{Op: "update", Filter: filter, Data: changes}); err != nil {
		return nil, err
	}
	updated, err := o.load()
	if err != nil {
		return nil, err
	}
	for _, doc := range updated {
		if matchesFilter(doc, filter) {
			return copyDoc(doc), nil
		}
	}
	return nil, nil
}

// Upsert finds the first document matching filter and merges data. If none
// match, inserts a merged document. Returns the post-upsert document.
func (o *Ops) Upsert(filter, data map[string]any) (map[string]any, error) {
	if err := store.AppendEvent(o.sess, o.name, store.CollectionEvent{Op: "upsert", Filter: filter, Data: data}); err != nil {
		return nil, err
	}
	docs, err := o.load()
	if err != nil {
		return nil, err
	}
	for _, doc := range docs {
		if matchesFilter(doc, filter) {
			return copyDoc(doc), nil
		}
	}
	if len(docs) > 0 {
		return copyDoc(docs[len(docs)-1]), nil
	}
	return nil, nil
}

// Delete removes the first document matching filter and returns it.
func (o *Ops) Delete(filter map[string]any) (map[string]any, error) {
	docs, err := o.load()
	if err != nil {
		return nil, err
	}
	var deleted map[string]any
	for _, doc := range docs {
		if matchesFilter(doc, filter) {
			deleted = copyDoc(doc)
			break
		}
	}
	if deleted == nil {
		return nil, nil
	}
	if err := store.AppendEvent(o.sess, o.name, store.CollectionEvent{Op: "delete", Filter: filter}); err != nil {
		return nil, err
	}
	return deleted, nil
}

// matchesFilter reports whether doc satisfies all filter fields (exact equality).
// A nil or empty filter matches everything.
func matchesFilter(doc, filter map[string]any) bool {
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

func copyDoc(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
