package collectionresponse

import (
	"encoding/json"
	"fmt"

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
	Matched     bool
	RootKind    models.RootKind
	Template    *ResolvedTemplate
	Doc         map[string]any   // object root
	Docs        []map[string]any // array root
	Filter      map[string]any
	RecordCount int
}

// TryMatch resolves the template's root kind, runs the primary query, and
// reports whether the response matches. It performs the query at most once;
// the result is carried by MatchResult for Render to reuse.
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
	res := &MatchResult{RootKind: rootKind, Template: tmpl}
	if sess == nil {
		return res, nil
	}

	bctx := &collection.BindingContext{Request: req}
	filter, err := collection.ResolveFilterMap(cr.Primary.FilterRules, bctx)
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
}

// Render runs additional mappers and fills the template with the primary
// match's document(s), returning the JSON-encoded body. Call only after
// TryMatch reported Matched.
func (s *Service) Render(cfg *models.ResponseConfig, match *MatchResult, req *collection.TypedRequestContext, sess store.SessionState) (*RenderResult, error) {
	cr := cfg.CollectionResponse
	mappers, mapperTraces, err := s.runAdditionalMappers(cr, match, req, sess)
	if err != nil {
		return &RenderResult{AdditionalMapperTraces: mapperTraces}, err
	}

	overrideMap := make(map[string]models.FieldOverride, len(cr.Overrides))
	for _, o := range cr.Overrides {
		overrideMap[o.TargetPath] = o
	}

	identity := match.Template.Source == TemplateSourceIdentity
	var value any
	var warnings []string

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
			value, warnings = fillIdentity(match.Doc, overrideMap, req, mappers)
		} else {
			value, warnings = FillDocument(match.Template.Value, match.Doc, overrideMap, req, mappers, cr.EffectiveFallbackToExample())
		}
	}

	body, err := json.Marshal(value)
	if err != nil {
		return &RenderResult{AdditionalMapperTraces: mapperTraces}, fmt.Errorf("marshal rendered collection response: %w", err)
	}
	return &RenderResult{Body: body, Warnings: warnings, AdditionalMapperTraces: mapperTraces}, nil
}

func (s *Service) runAdditionalMappers(cr *models.CollectionResponseConfig, match *MatchResult, req *collection.TypedRequestContext, sess store.SessionState) (map[string]any, []models.CollectionTrace, error) {
	if len(cr.AdditionalMappers) == 0 {
		return nil, nil, nil
	}
	mappers := make(map[string]any, len(cr.AdditionalMappers))
	traces := make([]models.CollectionTrace, 0, len(cr.AdditionalMappers))

	for _, m := range cr.AdditionalMappers {
		bctx := &collection.BindingContext{Request: req, Primary: match.Doc}
		filter, err := collection.ResolveFilterMap(m.FilterRules, bctx)
		if err != nil {
			return mappers, traces, err
		}

		trace := models.CollectionTrace{MappingName: m.OutputKey, CollectionName: m.CollectionName, OutputKey: m.OutputKey}
		ops := collection.NewOps(m.CollectionName, s.backend, sess)

		if m.Mode == models.QueryModeFindMany {
			trace.Operation = models.ColOpFindMany
			docs, err := ops.FindMany(filter)
			if err != nil {
				trace.Error = err.Error()
				traces = append(traces, trace)
				return mappers, traces, err
			}
			trace.RecordCount = len(docs)
			arr := make([]any, len(docs))
			for i, d := range docs {
				arr[i] = d
			}
			mappers[m.OutputKey] = arr
			traces = append(traces, trace)
			continue
		}

		trace.Operation = models.ColOpFindOne
		doc, err := ops.FindOne(filter)
		if err != nil {
			trace.Error = err.Error()
			traces = append(traces, trace)
			return mappers, traces, err
		}
		if doc != nil {
			trace.RecordCount = 1
		}
		mappers[m.OutputKey] = doc
		traces = append(traces, trace)
	}

	return mappers, traces, nil
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
	if tmpl.Source == TemplateSourceIdentity && cr.RootKind == "" {
		errs = append(errs, "rootKind is required because the operation defines no JSON response body for this status code")
	}
	if rootKind == models.RootKindArray {
		for i, m := range cr.AdditionalMappers {
			for j, f := range m.FilterRules {
				if f.Value.Source == models.ValueSourcePrimary {
					errs = append(errs, fmt.Sprintf(
						"additionalMappers[%d].filterRules[%d]: source \"primary\" requires the primary query to return a single document (object-rooted template)",
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
