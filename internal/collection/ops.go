package collection

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/store"
)

// Ops performs Go-level collection operations against a SessionState.
// The storage format matches store.CollectionBuiltin exactly: documents are
// stored as a JSON array under the key models.CollectionKeyPrefix + name.
// Data written by Ops is therefore visible to Starlark scripts that call
// store.collection(name) in the same session, and vice versa.
type Ops struct {
	sess store.SessionState
	name string
	key  string
}

// NewOps returns an Ops for the named collection in the given session.
func NewOps(name string, sess store.SessionState) *Ops {
	return &Ops{
		sess: sess,
		name: name,
		key:  models.CollectionKeyPrefix + name,
	}
}

// load reads all documents from the session store.
func (o *Ops) load() []map[string]any {
	raw, ok := o.sess.Get(o.key)
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []any:
		docs := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				docs = append(docs, m)
			}
		}
		return docs
	case []map[string]any:
		return v
	case string:
		// Deserialize from JSON string (Redis backend stores as JSON)
		var docs []map[string]any
		if err := json.Unmarshal([]byte(v), &docs); err == nil {
			return docs
		}
	}
	return nil
}

// save writes documents back to the session store.
func (o *Ops) save(docs []map[string]any) error {
	raw := make([]any, len(docs))
	for i, d := range docs {
		raw[i] = d
	}
	return o.sess.Set(o.key, raw)
}

// matchesFilter reports whether a document satisfies all filter fields (equality).
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

// copyDoc makes a shallow copy of a document.
func copyDoc(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// FindOne returns the first document matching filter, or nil if none found.
func (o *Ops) FindOne(filter map[string]any) (map[string]any, error) {
	for _, doc := range o.load() {
		if matchesFilter(doc, filter) {
			return copyDoc(doc), nil
		}
	}
	return nil, nil
}

// FindMany returns all documents matching filter (empty slice if none).
func (o *Ops) FindMany(filter map[string]any) ([]map[string]any, error) {
	var out []map[string]any
	for _, doc := range o.load() {
		if matchesFilter(doc, filter) {
			out = append(out, copyDoc(doc))
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, nil
}

// Insert appends a new document and returns it. If the document does not
// contain an "_id" field, a new UUID is assigned.
func (o *Ops) Insert(data map[string]any) (map[string]any, error) {
	doc := copyDoc(data)
	if _, hasID := doc["_id"]; !hasID {
		doc["_id"] = uuid.New().String()
	}
	docs := o.load()
	docs = append(docs, doc)
	if err := o.save(docs); err != nil {
		return nil, err
	}
	return copyDoc(doc), nil
}

// Update finds the first document matching filter, merges changes into it, and
// returns the post-update document. Returns nil if no document matched.
func (o *Ops) Update(filter, changes map[string]any) (map[string]any, error) {
	docs := o.load()
	var updated map[string]any
	for i, doc := range docs {
		if matchesFilter(doc, filter) {
			for k, v := range changes {
				docs[i][k] = v
			}
			updated = copyDoc(docs[i])
			break
		}
	}
	if updated == nil {
		return nil, nil
	}
	if err := o.save(docs); err != nil {
		return nil, err
	}
	return updated, nil
}

// Upsert finds the first document matching filter and merges data into it. If
// no document matches, a new one is inserted containing both filter fields and
// data fields. Returns the post-upsert document.
func (o *Ops) Upsert(filter, data map[string]any) (map[string]any, error) {
	docs := o.load()
	for i, doc := range docs {
		if matchesFilter(doc, filter) {
			for k, v := range data {
				docs[i][k] = v
			}
			result := copyDoc(docs[i])
			if err := o.save(docs); err != nil {
				return nil, err
			}
			return result, nil
		}
	}
	// No match — insert a merged document.
	newDoc := copyDoc(filter)
	for k, v := range data {
		newDoc[k] = v
	}
	if _, hasID := newDoc["_id"]; !hasID {
		newDoc["_id"] = uuid.New().String()
	}
	docs = append(docs, newDoc)
	if err := o.save(docs); err != nil {
		return nil, err
	}
	return copyDoc(newDoc), nil
}

// Delete removes the first document matching filter and returns it.
// Returns nil if no document matched.
func (o *Ops) Delete(filter map[string]any) (map[string]any, error) {
	docs := o.load()
	var deleted map[string]any
	remaining := docs[:0]
	for _, doc := range docs {
		if deleted == nil && matchesFilter(doc, filter) {
			deleted = copyDoc(doc)
		} else {
			remaining = append(remaining, doc)
		}
	}
	if deleted == nil {
		return nil, nil
	}
	if err := o.save(remaining); err != nil {
		return nil, err
	}
	return deleted, nil
}
