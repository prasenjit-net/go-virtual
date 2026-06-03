//go:build unit

package main

import (
	"errors"
	"log/slog"

	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/storage"
	gvstore "github.com/prasenjit/go-virtual/internal/store"
)

func newMongoStorageBackend(_ config.MongoConfig) (storage.Storage, error) {
	return nil, errors.New("MongoDB storage not available in unit build")
}

func newMongoGlobalStoreBackend(_ config.MongoConfig) (gvstore.GlobalStoreBackend, error) {
	return nil, errors.New("MongoDB global store not available in unit build")
}

func runMongoSetup(_ storage.Storage, _ *slog.Logger) {}
