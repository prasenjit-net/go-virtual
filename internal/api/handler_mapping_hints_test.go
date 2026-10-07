package api

import (
	"encoding/json"
	"github.com/prasenjit/go-virtual/internal/mappinghints"
	"github.com/prasenjit/go-virtual/internal/models"
	"net/http/httptest"
	"testing"
)

func TestMappingHintsContractObservationsAndOwnership(t *testing.T) {
	h, s, r, backend := setupCollectionResponseTestHandler(t)
	r.GET("/operations/:id/mapping-hints", h.MappingHints)
	spec, _ := s.GetSpec("spec-1")
	spec.Content = `{"openapi":"3.0.3","info":{"title":"t","version":"1"},"paths":{"/users/{id}":{"parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}},{"name":"state","in":"query","schema":{"type":"string","enum":["old"]}}],"get":{"parameters":[{"name":"state","in":"query","schema":{"type":"string","enum":["active","disabled"]}}],"requestBody":{"content":{"application/json":{"schema":{"type":"object","properties":{"addresses":{"type":"array","items":{"type":"object","properties":{"city":{"type":"string"}}}}}}}}},"responses":{"200":{"description":"ok"}}}}}}`
	if err := s.UpdateSpec(spec); err != nil {
		t.Fatal(err)
	}
	backend.SeedInsert("users", map[string]any{"name": "Alice", "details": map[string]any{"city": "Boston"}})
	h.tracingService.RecordTrace(&models.Trace{ID: "observed", OperationID: "op-user", Request: models.TraceRequest{Body: `{"custom":false}`, Headers: map[string][]string{"Authorization": {"secret"}, "X-Custom": {"observed"}}}})
	req := httptest.NewRequest("GET", "/operations/op-user/mapping-hints?collectionName=users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var catalog mappinghints.Catalog
	if err := json.Unmarshal(w.Body.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	found := map[string]mappinghints.Item{}
	for _, i := range catalog.Items {
		found[i.Source+":"+i.Key] = i
	}
	for _, key := range []string{"path:id", "query:state", "body:addresses.0.city", "body:custom", "header:X-Custom", "document:details.city"} {
		if _, ok := found[key]; !ok {
			t.Fatalf("missing %s: %s", key, w.Body.String())
		}
	}
	if found["query:state"].Values[0] != `"active"` {
		t.Fatal("parameter override ignored")
	}
	if len(found["header:Authorization"].Values) > 0 {
		t.Fatal("credential leaked")
	}
	docs, _ := backend.GetAll("users")
	if len(docs) != 1 || docs[0]["name"] != "Alice" {
		t.Fatal("hints mutated collection")
	}
	other := httptest.NewRecorder()
	r.ServeHTTP(other, httptest.NewRequest("GET", "/operations/op-users/mapping-hints?traceId=observed", nil))
	if other.Code != 400 {
		t.Fatalf("cross-operation trace allowed: %d", other.Code)
	}
	missing := httptest.NewRecorder()
	r.ServeHTTP(missing, httptest.NewRequest("GET", "/operations/missing/mapping-hints", nil))
	if missing.Code != 404 {
		t.Fatal("unknown operation accepted")
	}
}

func TestSpecMappingHintsCommonContract(t *testing.T) {
	h, _, r, _ := setupCollectionResponseTestHandler(t)
	r.GET("/specs/:id/mapping-hints", h.MappingHints)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/specs/spec-1/mapping-hints", nil))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var c mappinghints.Catalog
	json.Unmarshal(w.Body.Bytes(), &c)
	for _, hint := range c.Items {
		if hint.Source == "path" {
			t.Fatal("operation-only path exposed as common spec input")
		}
	}
}
