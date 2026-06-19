package store

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// ReplayEvents applies a slice of CollectionEvents in order on top of the
// provided base documents and returns the resulting slice. base is never
// mutated; a new slice is returned.
func ReplayEvents(base []map[string]any, events []CollectionEvent) []map[string]any {
	docs := make([]map[string]any, len(base))
	for i, d := range base {
		docs[i] = copyDoc(d)
	}

	for _, evt := range events {
		switch evt.Op {
		case "insert":
			doc := copyDoc(evt.Data)
			if _, ok := doc["_id"]; !ok {
				doc["_id"] = uuid.New().String()
			}
			docs = append(docs, doc)

		case "update":
			for i, doc := range docs {
				if matchesDocFilter(doc, evt.Filter) {
					for k, v := range evt.Data {
						docs[i][k] = v
					}
					break
				}
			}

		case "upsert":
			found := false
			for i, doc := range docs {
				if matchesDocFilter(doc, evt.Filter) {
					for k, v := range evt.Data {
						docs[i][k] = v
					}
					found = true
					break
				}
			}
			if !found {
				doc := copyDoc(evt.Filter)
				for k, v := range evt.Data {
					doc[k] = v
				}
				if _, ok := doc["_id"]; !ok {
					doc["_id"] = uuid.New().String()
				}
				docs = append(docs, doc)
			}

		case "delete":
			remaining := docs[:0]
			deleted := false
			for _, doc := range docs {
				if !deleted && matchesDocFilter(doc, evt.Filter) {
					deleted = true
				} else {
					remaining = append(remaining, doc)
				}
			}
			docs = remaining

		case "clear":
			docs = nil
		}
	}

	return docs
}

// LoadEvents reads the session's event log for the named collection.
// Returns nil when no events have been recorded.
func LoadEvents(sess SessionState, collection string) []CollectionEvent {
	raw, ok := sess.Get(CollectionEventKeyPrefix + collection)
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []CollectionEvent:
		return v
	case []any:
		// Deserialised from JSON (e.g. Redis round-trip stored as []any of maps)
		b, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		var evts []CollectionEvent
		if err := json.Unmarshal(b, &evts); err != nil {
			return nil
		}
		return evts
	case string:
		var evts []CollectionEvent
		if err := json.Unmarshal([]byte(v), &evts); err != nil {
			return nil
		}
		return evts
	}
	return nil
}

// AppendEvent appends one event to the session's collection event log.
func AppendEvent(sess SessionState, collection string, evt CollectionEvent) error {
	evts := LoadEvents(sess, collection)
	evts = append(evts, evt)
	return sess.Set(CollectionEventKeyPrefix+collection, evts)
}

// matchesDocFilter reports whether doc satisfies every field in filter (exact equality).
// A nil or empty filter matches everything.
func matchesDocFilter(doc, filter map[string]any) bool {
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

// copyDoc makes a shallow copy of a document map.
func copyDoc(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
