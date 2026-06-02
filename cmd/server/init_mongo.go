//go:build !unit

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/storage"
)

func initMongoIndexes(uri, database string) {
	fmt.Println()
	fmt.Print("Creating MongoDB indexes... ")
	cfg := config.MongoConfig{
		URI:                   uri,
		Database:              database,
		CollectionPrefix:      config.DefaultMongoCollectionPrefix,
		ConnectTimeoutSeconds: config.DefaultMongoConnectTimeoutSeconds,
		StartupRetrySeconds:   -1, // single attempt — init is interactive, not a retry loop
	}
	ms, err := storage.NewMongoStorage(cfg)
	if err != nil {
		fmt.Printf("warning: could not connect to MongoDB (%v)\n", err)
		fmt.Println("  Run `go-virtual init` again once MongoDB is reachable, or create indexes manually.")
		return
	}
	defer ms.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := ms.EnsureIndexes(ctx); err != nil {
		fmt.Printf("warning: index creation failed (%v)\n", err)
		fmt.Println("  The server will work without indexes but queries may be slower.")
		fmt.Println("  Grant createIndex privilege and re-run `go-virtual init` to create them.")
		return
	}
	fmt.Println("done.")
}
