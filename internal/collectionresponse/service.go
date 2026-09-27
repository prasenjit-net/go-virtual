package collectionresponse

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/prasenjit/go-virtual/internal/collection"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/parser"
	"github.com/prasenjit/go-virtual/internal/storage"
	"github.com/prasenjit/go-virtual/internal/store"
)

// Service resolves, matches, and renders Collection Responses.
type Service struct {
	store   storage.Storage
	backend store.CollectionBackend
	parser  *parser.Parser
}

// NewService creates a Service backed by the given storage (for spec lookup)
// and collection backend (for document reads).
func NewService(s storage.Storage, backend store.CollectionBackend) *Service {
	return &Service{store: s, backend: backend, parser: parser.NewParser()}
}

// MatchResult is the outcome of evaluating a Collection Response's primary
// query during matching.
type MatchResult struct {
	Matched          bool
	RootKind         models.RootKind
	Template         *ResolvedTemplate
	Doc              map[string]any   // object root
	Docs             []map[string]any // array root
	Filter           map[string]any
	RecordCount      int
	executionStarted bool
	prepared         bool
	mapperOutputs    map[string]any
	mapperTraces     []models.CollectionTrace
	primaryTrace     *models.CollectionTrace
}

// TryMatch resolves the template's root kind and reports collection eligibility.
// Insert/Upsert require no query; other modes query at most once and retain
// their result for execution and rendering. The caller evaluates conditions.
//
// A nil sess (no session-scoped collection state available) never matches —
// this mirrors how CollectionMapping steps are skipped without a session.
func (s *Service) TryMatch(op *models.Operation, cfg *models.ResponseConfig, req *collection.TypedRequestContext, sess store.SessionState) (*MatchResult, error) {
	cr := cfg.CollectionResponse
	if cr == nil {
		return nil, fmt.Errorf("response %s has no collection response configuration", cfg.ID)
	}

	tmpl, rootKind, err := s.resolveRootKind(op, cfg.StatusCode, cr)
	if err != nil {
		return nil, err
	}
	if (isMutation(cr.Primary.Mode) || cr.Primary.Mode == models.ColOpFindOne) && rootKind != models.RootKindObject {
		return nil, fmt.Errorf("primary.mode %s requires an object response root", cr.Primary.Mode)
	}
	if cr.Primary.Mode == models.ColOpFindMany && rootKind != models.RootKindArray {
		return nil, fmt.Errorf("primary.mode find-many requires an array response root")
	}
	res := &MatchResult{RootKind: rootKind, Template: tmpl}
	if sess == nil {
		return res, nil
	}

	// Conditions are evaluated by the response selector. These operations have
	// no read prerequisite; upsert filters are resolved only after selection.
	if cr.Primary.Mode == models.ColOpInsert || cr.Primary.Mode == models.ColOpUpsert {
		res.Matched = true
		return res, nil
	}

	bctx := &collection.BindingContext{Request: req}
	resolve := collection.ResolveFilterMap
	if cr.Primary.Mode == models.ColOpUpdate {
		resolve = collection.ResolveRequiredMap
	}
	filter, err := resolve(cr.Primary.FilterRules, bctx)
	if err != nil {
		return nil, err
	}
	res.Filter = filter

	ops := collection.NewOps(cr.Primary.CollectionName, s.backend, sess)
	if rootKind == models.RootKindArray {
		docs, err := ops.FindMany(filter)
		if err != nil {
			return nil, err
		}
		res.Docs = docs
		res.RecordCount = len(docs)
		res.Matched = len(docs) > 0 || cr.MatchOnEmpty
		return res, nil
	}

	doc, err := ops.FindOne(filter)
	if err != nil {
		return nil, err
	}
	res.Doc = doc
	if doc != nil {
		res.RecordCount = 1
	}
	res.Matched = doc != nil || cr.MatchOnEmpty
	return res, nil
}

// RenderResult is a rendered Collection Response body plus diagnostics.
type RenderResult struct {
	Body                   []byte
	Warnings               []string
	AdditionalMapperTraces []models.CollectionTrace
	PrimaryMapperTrace     *models.CollectionTrace
}

// Render fills the template from a selected operation result. For preview, an
// unprepared match runs only additional reads and reports skipped writes.
func (s *Service) Render(cfg *models.ResponseConfig, match *MatchResult, req *collection.TypedRequestContext, sess store.SessionState) (*RenderResult, error) {
	cr := cfg.CollectionResponse
	mappers, mapperTraces := match.mapperOutputs, match.mapperTraces
	if !match.prepared {
		// Rendering and preview may read, but never execute configured mutations.
		var err error
		mappers, mapperTraces, err = s.runAdditionalMappers(cr, match, req, sess, false)
		if err != nil {
			return &RenderResult{AdditionalMapperTraces: mapperTraces, PrimaryMapperTrace: match.primaryTrace}, err
		}
	}

	overrideMap := make(map[string]models.FieldOverride, len(cr.Overrides))
	for _, o := range cr.Overrides {
		overrideMap[o.TargetPath] = o
	}

	identity := match.Template.Source == TemplateSourceIdentity
	var value any
	var warnings []string
	if !match.prepared && isMutation(cr.Primary.Mode) {
		warnings = append(warnings, "Preview does not execute the main "+string(cr.Primary.Mode)+"; no write result is simulated.")
	}
	if !match.prepared {
		for _, m := range cr.AdditionalMappers {
			if isMutation(m.Mode) {
				warnings = append(warnings, "Preview skipped mutation mapper: "+m.OutputKey)
			}
		}
	}

	switch match.RootKind {
	case models.RootKindArray:
		item := match.Template.ItemTemplate()
		items := make([]any, 0, len(match.Docs))
		for _, d := range match.Docs {
			var v any
			var w []string
			if identity {
				v, w = fillIdentity(d, overrideMap, req, mappers)
			} else {
				v, w = FillDocument(item, d, overrideMap, req, mappers, cr.EffectiveFallbackToExample())
			}
			items = append(items, v)
			warnings = append(warnings, w...)
		}
		value = items

	default: // object root
		if match.Doc == nil {
			value = nil
		} else if identity {
			var w []string
			value, w = fillIdentity(match.Doc, overrideMap, req, mappers)
			warnings = append(warnings, w...)
		} else {
			var w []string
			value, w = FillDocument(match.Template.Value, match.Doc, overrideMap, req, mappers, cr.EffectiveFallbackToExample())
			warnings = append(warnings, w...)
		}
	}

	body, err := json.Marshal(value)
	if err != nil {
		return &RenderResult{AdditionalMapperTraces: mapperTraces, PrimaryMapperTrace: match.primaryTrace}, fmt.Errorf("marshal rendered collection response: %w", err)
	}
	return &RenderResult{Body: body, Warnings: warnings, AdditionalMapperTraces: mapperTraces, PrimaryMapperTrace: match.primaryTrace}, nil
}

// ExecuteSelected performs operations once after matching. Render remains read-only.
// The returned snapshot carries partial diagnostics even on failure.
func (s *Service) ExecuteSelected(cfg *models.ResponseConfig, match *MatchResult, req *collection.TypedRequestContext, sess store.SessionState) (*MatchResult, error) {
	if match.executionStarted || match.prepared {
		return match, fmt.Errorf("collection response operations already executed")
	}
	match.executionStarted = true
	result := *match
	result.prepared = true
	if !match.Matched || sess == nil {
		return &result, fmt.Errorf("collection response execution requires a selected response and session")
	}
	cr := cfg.CollectionResponse
	if cr.Primary.Mode == models.ColOpUpdate {
		start := time.Now()
		trace := &models.CollectionTrace{MappingName: "Primary mapper", CollectionName: cr.Primary.CollectionName, Operation: models.ColOpUpdate}
		result.primaryTrace = trace
		var err error
		if match.Doc != nil {
			var data map[string]any
			data, err = collection.ResolveRequiredMap(cr.Primary.DataRules, &collection.BindingContext{Request: req, Primary: match.Doc})
			if err == nil {
				id, exists := match.Doc["_id"]
				if !exists || id == nil || fmt.Sprint(id) == "" {
					err = fmt.Errorf("selected document has no identity")
				} else {
					result.Doc, err = collection.NewOps(cr.Primary.CollectionName, s.backend, sess).Update(map[string]any{"_id": id}, data)
					if err == nil && result.Doc == nil {
						err = fmt.Errorf("selected update target no longer exists")
					}
				}
			}
			if err == nil {
				trace.RecordCount = 1
			}
		}
		trace.DurationMs = float64(time.Since(start).Microseconds()) / 1000
		if err != nil {
			trace.Error = err.Error()
			return &result, err
		}
	}
	if cr.Primary.Mode == models.ColOpInsert || cr.Primary.Mode == models.ColOpUpsert {
		start := time.Now()
		trace := &models.CollectionTrace{MappingName: "Primary mapper", CollectionName: cr.Primary.CollectionName, Operation: cr.Primary.Mode}
		result.primaryTrace = trace
		ctx := &collection.BindingContext{Request: req}
		data, err := collection.ResolveRequiredMap(cr.Primary.DataRules, ctx)
		if err == nil {
			ops := collection.NewOps(cr.Primary.CollectionName, s.backend, sess)
			if cr.Primary.Mode == models.ColOpInsert {
				result.Doc, err = ops.Insert(data)
			} else {
				var filter map[string]any
				filter, err = collection.ResolveRequiredMap(cr.Primary.FilterRules, ctx)
				result.Filter = filter
				if err == nil {
					result.Doc, err = ops.Upsert(filter, data)
				}
			}
		}
		trace.DurationMs = float64(time.Since(start).Microseconds()) / 1000
		if err != nil {
			trace.Error = err.Error()
			return &result, err
		}
		if result.Doc != nil {
			trace.RecordCount = 1
			result.RecordCount = 1
		}
	}
	outputs, traces, err := s.runAdditionalMappers(cr, &result, req, sess, true)
	result.mapperOutputs, result.mapperTraces = outputs, traces
	return &result, err
}

func isMutation(mode models.CollectionOpType) bool {
	return mode == models.ColOpInsert || mode == models.ColOpUpdate || mode == models.ColOpUpsert || mode == models.ColOpDelete
}

func (s *Service) runAdditionalMappers(cr *models.CollectionResponseConfig, match *MatchResult, req *collection.TypedRequestContext, sess store.SessionState, executeWrites bool) (map[string]any, []models.CollectionTrace, error) {
	outputs := make(map[string]any, len(cr.AdditionalMappers))
	traces := make([]models.CollectionTrace, 0, len(cr.AdditionalMappers))
	// Preflight bindings before any additional writes. Primary is the main operation result.
	filters := make([]map[string]any, len(cr.AdditionalMappers))
	data := make([]map[string]any, len(cr.AdditionalMappers))
	for i, m := range cr.AdditionalMappers {
		if isMutation(m.Mode) && !executeWrites {
			continue
		}
		ctx := &collection.BindingContext{Request: req, Primary: match.Doc}
		resolve := collection.ResolveFilterMap
		if isMutation(m.Mode) {
			resolve = collection.ResolveRequiredMap
		}
		var err error
		filters[i], err = resolve(m.FilterRules, ctx)
		if err == nil {
			data[i], err = collection.ResolveRequiredMap(m.DataRules, ctx)
		}
		if err != nil {
			traces = append(traces, models.CollectionTrace{MappingName: m.OutputKey, OutputKey: m.OutputKey, CollectionName: m.CollectionName, Operation: m.Mode, Error: err.Error()})
			return outputs, traces, err
		}
	}
	for i, m := range cr.AdditionalMappers {
		if isMutation(m.Mode) && !executeWrites {
			outputs[m.OutputKey] = nil
			continue
		}
		start := time.Now()
		trace := models.CollectionTrace{MappingName: m.OutputKey, OutputKey: m.OutputKey, CollectionName: m.CollectionName, Operation: m.Mode}
		ops := collection.NewOps(m.CollectionName, s.backend, sess)
		var doc map[string]any
		var err error
		switch m.Mode {
		case models.ColOpFindMany:
			var docs []map[string]any
			docs, err = ops.FindMany(filters[i])
			arr := make([]any, len(docs))
			for j, d := range docs {
				arr[j] = d
			}
			outputs[m.OutputKey] = arr
			trace.RecordCount = len(docs)
		case models.ColOpFindOne:
			doc, err = ops.FindOne(filters[i])
		case models.ColOpInsert:
			doc, err = ops.Insert(data[i])
		case models.ColOpUpdate:
			doc, err = ops.Update(filters[i], data[i])
		case models.ColOpUpsert:
			doc, err = ops.Upsert(filters[i], data[i])
		case models.ColOpDelete:
			doc, err = ops.Delete(filters[i])
		default:
			err = fmt.Errorf("unsupported mapper operation %q", m.Mode)
		}
		if m.Mode != models.ColOpFindMany {
			if doc != nil {
				outputs[m.OutputKey] = doc
				trace.RecordCount = 1
			} else {
				outputs[m.OutputKey] = nil
			}
		}
		trace.DurationMs = float64(time.Since(start).Microseconds()) / 1000
		if err != nil {
			trace.Error = err.Error()
		}
		traces = append(traces, trace)
		if err != nil {
			return outputs, traces, err
		}
	}
	return outputs, traces, nil
}

// resolveRootKind resolves the operation's spec template for cfg's status
// code and derives the query root kind: the template's own shape, or — in
// identity mode — the response's explicit RootKind (defaulting to object).
func (s *Service) resolveRootKind(op *models.Operation, statusCode int, cr *models.CollectionResponseConfig) (*ResolvedTemplate, models.RootKind, error) {
	specContent, err := s.specContentForOperation(op)
	if err != nil {
		return nil, "", err
	}
	tmpl, err := ResolveTemplate(s.parser, specContent, op, statusCode, cr.TemplateRef)
	if err != nil {
		return nil, "", err
	}
	if tmpl.Source == TemplateSourceIdentity {
		root := cr.RootKind
		if root == "" {
			root = models.RootKindObject
		}
		tmpl.Root = root
		return tmpl, root, nil
	}
	return tmpl, tmpl.Root, nil
}

func (s *Service) specContentForOperation(op *models.Operation) (string, error) {
	spec, err := s.store.GetSpec(op.SpecID)
	if err != nil {
		return "", err
	}
	return spec.Content, nil
}

// ValidateAgainstOperation performs the cross-checks that require resolving
// the operation's spec response schema:
//   - identity mode (no JSON body defined for the status code) requires an
//     explicit RootKind
//   - a "primary" filter binding in an additional mapper requires the
//     primary query to return a single document (an object-rooted template)
func (s *Service) ValidateAgainstOperation(op *models.Operation, cfg *models.ResponseConfig) ([]string, error) {
	cr := cfg.CollectionResponse
	if cr == nil {
		return nil, nil
	}
	tmpl, rootKind, err := s.resolveRootKind(op, cfg.StatusCode, cr)
	if err != nil {
		return nil, err
	}

	var errs []string
	if (isMutation(cr.Primary.Mode) || cr.Primary.Mode == models.ColOpFindOne) && rootKind != models.RootKindObject {
		errs = append(errs, "primary.mode requires an object response root")
	}
	if cr.Primary.Mode == models.ColOpFindMany && rootKind != models.RootKindArray {
		errs = append(errs, "primary.mode find-many requires an array response root")
	}
	if tmpl.Source == TemplateSourceIdentity && cr.RootKind == "" {
		errs = append(errs, "rootKind is required because the operation defines no JSON response body for this status code")
	}
	if rootKind == models.RootKindArray {
		for i, m := range cr.AdditionalMappers {
			for j, f := range append(append([]models.CollectionFilter(nil), m.FilterRules...), m.DataRules...) {
				if f.Value.Source == models.ValueSourcePrimary {
					errs = append(errs, fmt.Sprintf(
						"additionalMappers[%d].bindings[%d]: source \"primary\" requires the primary query to return a single document (object-rooted template)",
						i, j,
					))
				}
			}
		}
	}
	return errs, nil
}

// ResolveTemplateFor is a read-only helper for the preview endpoint and UI:
// it resolves the template that a Collection Response would use without
// running any query.
func (s *Service) ResolveTemplateFor(op *models.Operation, statusCode int, templateRef string) (*ResolvedTemplate, error) {
	specContent, err := s.specContentForOperation(op)
	if err != nil {
		return nil, err
	}
	return ResolveTemplate(s.parser, specContent, op, statusCode, templateRef)
}

// RenderTrace keeps operation diagnostics available when execution or filling fails.
func (m *MatchResult) RenderTrace(statusCode int) *models.CollectionResponseRenderTrace {
	return &models.CollectionResponseRenderTrace{TemplateStatusCode: statusCode, TemplateSource: string(m.Template.Source), PrimaryMapper: m.primaryTrace, AdditionalMappers: m.mapperTraces}
}
