//go:build !unit

package main

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/logging"
	"github.com/prasenjit/go-virtual/internal/proxy"
	gvsync "github.com/prasenjit/go-virtual/internal/sync"
	gvstore "github.com/prasenjit/go-virtual/internal/store"
)

// routeCollections are the logical collection names (without prefix) whose
// mutations require a route-table reload.
var routeCollections = []string{"specs", "operations"}

// storeCollections are the logical collection names whose mutations must be
// propagated to the MongoGlobalStore in-memory cache.
var storeCollections = []string{"global_store"}

// startMongoSync starts the cross-instance synchronisation subsystem for
// MongoDB deployments.  It selects the appropriate watcher strategy (change
// streams or polling) based on cfg.Sync.Mode, wires it to the proxy engine's
// route-reload channel and to the MongoGlobalStore cache, then launches
// everything in background goroutines that stop when ctx is cancelled.
//
// A nil globalStore (or a globalStore that is not a *gvstore.MongoGlobalStore)
// disables store-entry synchronisation; route synchronisation still proceeds.
func startMongoSync(
	ctx context.Context,
	mongoCfg config.MongoConfig,
	engine *proxy.Engine,
	globalStore gvstore.GlobalStoreBackend,
) {
	logger := logging.Logger("sync")

	mode := mongoCfg.Sync.Mode
	pollInterval := time.Duration(mongoCfg.Sync.PollIntervalSeconds) * time.Second

	if mode == config.MongoSyncModeOff {
		logger.Info("Cross-instance sync disabled by configuration",
			"event", "sync_disabled",
		)
		return
	}

	// Open a dedicated MongoDB connection for the watcher (change streams
	// use a long-lived cursor that should not share the storage connection).
	db, err := openWatcherDB(mongoCfg)
	if err != nil {
		logger.Error("Failed to open watcher MongoDB connection; sync disabled",
			"event", "sync_db_open_failed",
			"error", err,
		)
		return
	}

	// Route-change notification channel. Buffered so the watcher goroutine
	// never blocks on a slow reload.
	routeNotifyCh := make(chan struct{}, 16)
	engine.StartRouteSync(ctx, routeNotifyCh)

	// Build the handler that converts ChangeEvents into local actions.
	mongoGS, _ := globalStore.(*gvstore.MongoGlobalStore)
	handler := buildChangeHandler(routeNotifyCh, mongoGS)

	allCollections := append(routeCollections, storeCollections...)

	switch mode {
	case config.MongoSyncModeChangeStream:
		watcher := gvsync.NewMongoChangeWatcher(db, mongoCfg.CollectionPrefix, allCollections)
		go runWatcher(ctx, watcher, handler, logger, "change_stream")

	case config.MongoSyncModePolling:
		watcher := gvsync.NewPollingWatcher(db, mongoCfg.CollectionPrefix, allCollections, pollInterval)
		go runWatcher(ctx, watcher, handler, logger, "polling")

	default: // MongoSyncModeAuto
		// Try change streams; fall back to polling if not supported.
		csWatcher := gvsync.NewMongoChangeWatcher(db, mongoCfg.CollectionPrefix, allCollections)
		go func() {
			err := csWatcher.Watch(ctx, handler)
			if err == nil || ctx.Err() != nil {
				return // normal shutdown
			}
			if errors.Is(err, gvsync.ErrChangeStreamsNotSupported) {
				logger.Warn("Change streams not supported; falling back to polling",
					"event", "sync_fallback_to_polling",
					"poll_interval", pollInterval,
				)
				pw := gvsync.NewPollingWatcher(db, mongoCfg.CollectionPrefix, allCollections, pollInterval)
				runWatcher(ctx, pw, handler, logger, "polling")
				return
			}
			logger.Error("Change stream watcher terminated unexpectedly",
				"event", "sync_watcher_failed",
				"error", err,
			)
		}()
		logger.Info("Cross-instance sync started (auto mode: trying change streams)",
			"event", "sync_started",
			"mode", "auto",
		)
		return
	}

	logger.Info("Cross-instance sync started",
		"event", "sync_started",
		"mode", string(mode),
		"poll_interval_seconds", mongoCfg.Sync.PollIntervalSeconds,
	)
}

// buildChangeHandler returns a gvsync.Handler that routes events to the
// appropriate local subsystem.
func buildChangeHandler(
	routeNotifyCh chan<- struct{},
	mongoGS *gvstore.MongoGlobalStore,
) gvsync.Handler {
	return func(evt gvsync.ChangeEvent) {
		switch evt.Collection {
		case "specs", "operations":
			// Non-blocking send; the debounce loop in the engine will batch
			// rapid bursts into a single reload.
			select {
			case routeNotifyCh <- struct{}{}:
			default:
			}

		case "global_store":
			if mongoGS != nil {
				mongoGS.ApplyChangeEvent(string(evt.Operation), evt.DocumentID)
			}
		}
	}
}

// runWatcher calls watcher.Watch and logs any unexpected error.
func runWatcher(ctx context.Context, watcher gvsync.ChangeWatcher, handler gvsync.Handler, logger interface {
	Error(string, ...any)
	Info(string, ...any)
}, label string) {
	logger.Info("Change watcher goroutine started", "event", "sync_watcher_started", "type", label)
	err := watcher.Watch(ctx, handler)
	if err != nil && ctx.Err() == nil {
		logger.Error("Change watcher exited with error",
			"event", "sync_watcher_error",
			"type", label,
			"error", err,
		)
	}
}

// openWatcherDB creates a separate *mongo.Database for the sync watcher so
// it does not share connection-pool resources with the storage layer.
func openWatcherDB(cfg config.MongoConfig) (*mongo.Database, error) {
	timeout := time.Duration(cfg.ConnectTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	opts := options.Client().ApplyURI(cfg.URI).SetConnectTimeout(timeout)
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, err
	}
	return client.Database(cfg.Database), nil
}
