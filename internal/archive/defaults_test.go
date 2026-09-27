package archive_test

import (
	"encoding/json"
	"github.com/prasenjit/go-virtual/internal/archive"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/storage"
	"testing"
)

func TestCollectionResponseDefaultsArchiveRoundTrip(t *testing.T) {
	src := storage.NewMemoryStorage()
	seedStorage(t, src)
	cfg, err := src.GetResponseConfig("resp-001")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Kind = models.ResponseKindCollection
	cfg.Body = ""
	cfg.CollectionResponse = &models.CollectionResponseConfig{Primary: models.CollectionQuery{CollectionName: "pets", FilterRules: []models.CollectionFilter{{TargetPath: "id", Value: models.ValueBinding{Source: models.ValueSourceQuery, Key: "id", DefaultValue: json.RawMessage(`null`)}}}}}
	if err := src.UpdateResponseConfig(cfg); err != nil {
		t.Fatal(err)
	}
	raw, _, err := archive.BuildZIP("defaults", src, newTestStore(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	dst := storage.NewMemoryStorage()
	result, err := archive.ApplyZIP(raw, archive.RestoreOptions{}, dst, newTestStore(t), nil)
	if err != nil || len(result.Errors) > 0 {
		t.Fatalf("restore: %v %v", result, err)
	}
	cfg, err = dst.GetResponseConfig("resp-001")
	if err != nil {
		t.Fatal(err)
	}
	if string(cfg.CollectionResponse.Primary.FilterRules[0].Value.DefaultValue) != "null" {
		t.Fatal("typed null default lost")
	}
}

func TestStandaloneMapperDefaultFilePersistence(t *testing.T) {
	dir := t.TempDir()
	src, err := storage.NewFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	seedStorage(t, src)
	empty := ""
	mapping := &models.CollectionMapping{ID: "defaults-map", SpecID: "spec-001", CollectionName: "pets", OutputKey: "pet", Operation: models.ColOpFindOne, FilterRules: []models.FieldMappingRule{{TargetField: "id", SourceType: "query", SourceKey: "id", DefaultValue: &empty}}}
	if err := src.CreateCollectionMapping(mapping); err != nil {
		t.Fatal(err)
	}
	dst, err := storage.NewFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := dst.GetCollectionMapping(mapping.ID)
	if err != nil || saved.FilterRules[0].DefaultValue == nil || *saved.FilterRules[0].DefaultValue != "" {
		t.Fatalf("default lost: %+v %v", saved, err)
	}
}
