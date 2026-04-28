//go:build !unit

package main

import (
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
