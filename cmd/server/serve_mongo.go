//go:build !unit

package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/storage"
	gvstore "github.com/prasenjit/go-virtual/internal/store"
)

func newMongoStorageBackend(cfg config.MongoConfig) (storage.Storage, error) {
	return storage.NewMongoStorage(cfg)
}

func newMongoGlobalStoreBackend(cfg config.MongoConfig) (gvstore.GlobalStoreBackend, error) {
	return gvstore.NewMongoGlobalStoreFromConfig(cfg)
}

func newMongoCollectionBackend(cfg config.MongoConfig) (gvstore.CollectionBackend, error) {
	return gvstore.NewMongoCollectionBackendFromConfig(cfg)
}

func runMongoSetup(store storage.Storage, log *slog.Logger) {
	ms, ok := store.(*storage.MongoStorage)
	if !ok {
		return
	}
	log.Info("Running MongoDB setup: ensuring indexes", "event", "mongo_setup_start")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := ms.EnsureIndexes(ctx); err != nil {
		log.Warn("MongoDB index setup failed — server will start anyway but queries may be slow",
			"event", "mongo_setup_failed", "error", err)
		return
	}
	log.Info("MongoDB indexes ready", "event", "mongo_setup_done")
}
