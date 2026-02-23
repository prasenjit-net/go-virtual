package models

import "time"

// StoreEntry represents one key-value pair in the global store.
// Values can be any JSON-serialisable type: string, number, boolean, array, or object.
type StoreEntry struct {
	Key       string    `json:"key"`
	Value     any       `json:"value"`     // Any JSON-serialisable type
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// SessionInfo is a read-only summary of a live session, used by the Admin API.
type SessionInfo struct {
	ID          string         `json:"id"`
	CreatedAt   time.Time      `json:"createdAt"`
	LastActive  time.Time      `json:"lastActive"`
	EntryCount  int            `json:"entryCount"`
	StoreSnapshot map[string]any `json:"storeSnapshot,omitempty"`
}

// SessionTrace records session identification and store access during a single request.
type SessionTrace struct {
	ID          string             `json:"id"`
	IsNew       bool               `json:"isNew"`
	StoreAccess []StoreAccessEvent `json:"storeAccess,omitempty"`
}

// StoreAccessEvent records a single store operation performed by a script.
type StoreAccessEvent struct {
	Op    string `json:"op"`    // "get", "set", "delete", "has", "keys"
	Key   string `json:"key,omitempty"`
	Value any    `json:"value,omitempty"`
}
