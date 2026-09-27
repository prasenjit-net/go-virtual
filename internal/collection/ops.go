package collection

import (
	"fmt"
	"github.com/google/uuid"

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
	return o.insert(data, "insert")
}

func (o *Ops) insert(data map[string]any, operation string) (map[string]any, error) {
	doc := copyDoc(data)
	if _, ok := doc["_id"]; !ok {
		doc["_id"] = uuid.New().String()
	}
	if doc["_id"] == nil || fmt.Sprint(doc["_id"]) == "" {
		return nil, fmt.Errorf("document identity is required")
	}
	existing, err := o.FindOne(map[string]any{"_id": doc["_id"]})
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("duplicate document identity %v", doc["_id"])
	}
	if err := store.AppendEvent(o.sess, o.name, store.CollectionEvent{Op: operation, Filter: map[string]any{"_id": doc["_id"]}, Data: doc}); err != nil {
		return nil, err
	}
	return copyDoc(doc), nil
}

// Update finds the first document matching filter, merges changes into it, and
// returns the post-update document. Returns nil if no document matched.
func (o *Ops) Update(filter, changes map[string]any) (map[string]any, error) {
	return o.update(filter, changes, "update")
}

func (o *Ops) update(filter, changes map[string]any, operation string) (map[string]any, error) {
	if _, changesID := changes["_id"]; changesID {
		return nil, fmt.Errorf("_id cannot be changed")
	}
	docs, err := o.load()
	if err != nil {
		return nil, err
	}
	for _, doc := range docs {
		if !matchesFilter(doc, filter) {
			continue
		}
		id, ok := doc["_id"]
		if !ok || id == nil || fmt.Sprint(id) == "" {
			return nil, fmt.Errorf("update target has no identity")
		}
		identity := map[string]any{"_id": id}
		count := 0
		for _, candidate := range docs {
			if matchesFilter(candidate, identity) {
				count++
			}
		}
		if count != 1 {
			return nil, fmt.Errorf("update target has ambiguous identity %v", id)
		}
		if err := store.AppendEvent(o.sess, o.name, store.CollectionEvent{Op: operation, Filter: identity, Data: copyDoc(changes)}); err != nil {
			return nil, err
		}
		result := copyDoc(doc)
		for key, value := range changes {
			result[key] = value
		}
		return result, nil
	}
	return nil, nil
}

// Upsert updates the first match or inserts a merged document with a stable ID.
func (o *Ops) Upsert(filter, data map[string]any) (map[string]any, error) {
	if _, changesID := data["_id"]; changesID {
		return nil, fmt.Errorf("_id cannot be changed")
	}
	doc, err := o.update(filter, data, "upsert")
	if err != nil || doc != nil {
		return doc, err
	}
	merged := copyDoc(filter)
	for key, value := range data {
		merged[key] = value
	}
	return o.insert(merged, "upsert")
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
