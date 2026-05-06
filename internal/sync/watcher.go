package sync

import "context"

// ChangeOperation represents the type of mutation that occurred on a document.
type ChangeOperation string

const (
	ChangeOpInsert  ChangeOperation = "insert"
	ChangeOpUpdate  ChangeOperation = "update"
	ChangeOpReplace ChangeOperation = "replace"
	ChangeOpDelete  ChangeOperation = "delete"
)

// ChangeEvent describes a single mutation that occurred in the backing store
// and needs to be propagated to other in-memory caches within the same
// process or across horizontally-scaled instances.
type ChangeEvent struct {
	// Collection is the logical name of the changed collection (e.g. "specs",
	// "operations", "global_store"). It does NOT include a collection prefix.
	Collection string
	// Operation is the kind of change that occurred.
	Operation ChangeOperation
	// DocumentID is the _id of the affected document.
	DocumentID string
	// FullDoc holds the full deserialized document for inserts and replaces.
	// It is nil for updates and deletes.
	FullDoc map[string]any
}

// Handler is the callback invoked for every incoming ChangeEvent.
// Implementations must be safe for concurrent use and must not block for
// extended periods; hand off slow work to a separate goroutine if needed.
type Handler func(event ChangeEvent)

// ChangeWatcher watches one or more collections for mutations and delivers
// ChangeEvents to a registered Handler.
type ChangeWatcher interface {
	// Watch starts watching and blocks until ctx is cancelled or a fatal error
	// occurs. It must call handler for every event it observes. Transient
	// errors (e.g. network blips) should be retried internally rather than
	// causing Watch to return. Watch should return only when ctx is done or
	// when a permanent error (e.g. unsupported deployment) occurs.
	Watch(ctx context.Context, handler Handler) error
}

// ErrChangeStreamsNotSupported is returned by MongoChangeWatcher.Watch when
// the target MongoDB deployment does not support change streams (i.e. a
// standalone mongod without a replica-set configuration).  Callers can detect
// this sentinel and fall back to PollingWatcher.
var ErrChangeStreamsNotSupported = changeStreamsNotSupportedError{}

type changeStreamsNotSupportedError struct{}

func (changeStreamsNotSupportedError) Error() string {
	return "mongodb change streams not supported: target deployment is not a replica set or sharded cluster"
}
