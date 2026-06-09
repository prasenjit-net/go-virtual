package collection

import (
	"context"
	"sort"
	"time"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/store"
)

// Executor runs CollectionMappings for a ResponseConfig against the active
// session state and returns the output map and execution traces.
type Executor struct {
	store storage.Storage
}

// NewExecutor creates an Executor backed by the given storage.
func NewExecutor(s storage.Storage) *Executor {
	return &Executor{store: s}
}

// Run executes all enabled CollectionMappings for responseConfigID in Order
// order. sess must be the same SessionState that is passed to the script
// engine for this request so that mapper writes are visible to scripts and
// vice versa. Returns a map of outputKey → result document(s), and a trace
// slice.
func (e *Executor) Run(
	_ context.Context,
	responseConfigID string,
	req *RequestContext,
	sess store.SessionState,
) (map[string]any, []models.CollectionTrace, error) {
	mappings, err := e.store.GetCollectionMappingsByResponse(responseConfigID)
	if err != nil {
		return nil, nil, err
	}

	sort.Slice(mappings, func(i, j int) bool {
		return mappings[i].Order < mappings[j].Order
	})

	output := make(map[string]any)
	var traces []models.CollectionTrace

	for _, m := range mappings {
		if !m.Enabled {
			continue
		}

		start := time.Now()
		trace := models.CollectionTrace{
			MappingID:      m.ID,
			MappingName:    m.Name,
			CollectionName: m.CollectionName,
			Operation:      m.Operation,
			OutputKey:      m.OutputKey,
		}

		result, count, execErr := e.execute(m, req, sess)
		trace.DurationMs = float64(time.Since(start).Microseconds()) / 1000.0
		trace.RecordCount = count
		if execErr != nil {
			trace.Error = execErr.Error()
		} else if m.OutputKey != "" {
			output[m.OutputKey] = result
		}

		traces = append(traces, trace)
	}

	return output, traces, nil
}

// execute runs a single mapping and returns its result, record count, and any error.
func (e *Executor) execute(
	m *models.CollectionMapping,
	req *RequestContext,
	sess store.SessionState,
) (any, int, error) {
	ops := NewOps(m.CollectionName, sess)
	filter := ResolveMap(m.FilterRules, req)
	data := ResolveMap(m.DataRules, req)

	switch m.Operation {
	case models.ColOpFindOne:
		doc, err := ops.FindOne(filter)
		if err != nil {
			return nil, 0, err
		}
		if doc == nil {
			return nil, 0, nil
		}
		return doc, 1, nil

	case models.ColOpFindMany:
		docs, err := ops.FindMany(filter)
		if err != nil {
			return nil, 0, err
		}
		return docs, len(docs), nil

	case models.ColOpInsert:
		doc, err := ops.Insert(data)
		if err != nil {
			return nil, 0, err
		}
		return doc, 1, nil

	case models.ColOpUpdate:
		doc, err := ops.Update(filter, data)
		if err != nil {
			return nil, 0, err
		}
		if doc == nil {
			return nil, 0, nil
		}
		return doc, 1, nil

	case models.ColOpUpsert:
		doc, err := ops.Upsert(filter, data)
		if err != nil {
			return nil, 0, err
		}
		return doc, 1, nil

	case models.ColOpDelete:
		doc, err := ops.Delete(filter)
		if err != nil {
			return nil, 0, err
		}
		if doc == nil {
			return nil, 0, nil
		}
		return doc, 1, nil

	default:
		return nil, 0, nil
	}
}
