package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
	"github.com/prasenjit/go-virtual/internal/mappinghints"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/store"
)

// MappingHints returns metadata only; no runtime operations are executed.
func (h *Handler) MappingHints(c *gin.Context) {
	catalog := mappinghints.New()
	var ops []*models.Operation
	var specID string
	if strings.Contains(c.FullPath(), "/operations/") {
		op, err := h.store.GetOperation(c.Param("id"))
		if err != nil {
			c.JSON(404, gin.H{"error": "operation not found"})
			return
		}
		ops = []*models.Operation{op}
		specID = op.SpecID
	} else {
		specID = c.Param("id")
		var err error
		ops, err = h.store.GetOperationsBySpec(specID)
		if err != nil {
			c.JSON(404, gin.H{"error": "spec not found"})
			return
		}
	}
	spec, err := h.store.GetSpec(specID)
	if err != nil {
		c.JSON(404, gin.H{"error": "spec not found"})
		return
	}
	if operationID := c.Query("operationId"); operationID != "" {
		selected, err := h.store.GetOperation(operationID)
		if err != nil || selected.SpecID != specID {
			c.JSON(400, gin.H{"error": "operation does not belong to this spec"})
			return
		}
		ops = []*models.Operation{selected}
	}
	doc, err := openapi3.NewLoader().LoadFromData([]byte(spec.Content))
	if err != nil {
		catalog.Warnings = append(catalog.Warnings, "Spec metadata unavailable")
	}
	// Spec-wide request hints are the intersection of operation contracts.
	for i, op := range ops {
		current := mappinghints.New()
		if doc != nil {
			addRequestHints(current, doc, op)
		}
		if i == 0 {
			catalog.Items = current.Items
		} else {
			keep := catalog.Items[:0]
			for _, hint := range catalog.Items {
				for _, candidate := range current.Items {
					if hint.Source == candidate.Source && hint.Key == candidate.Key {
						hint.Required = hint.Required && candidate.Required
						keep = append(keep, hint)
						break
					}
				}
			}
			catalog.Items = keep
		}
	}
	producers := map[string]mappinghints.Item{}
	addSteps := func(steps []models.PipelineStep, scope string) {
		for _, step := range steps {
			base := mappinghints.Item{Scope: scope, Order: step.Order, Origin: "configured", Conditional: true}
			switch {
			case step.Script != nil && step.Script.Enabled:
				script, err := h.store.GetScript(step.Script.ScriptID)
				if err != nil || !script.Enabled {
					continue
				}
				base.Source = "script"
				base.Key = step.Script.OutputKey
				base.ProducerID = step.Script.ID
				base.Type = "unknown"
				catalog.Add(base)
				producers["script:"+step.Script.ID] = base
			case step.Validation != nil && step.Validation.Enabled:
				rule := step.Validation
				base.Source = "validation"
				base.Key = rule.Name
				base.ProducerID = rule.ID
				base.Type = "string"
				producers["validation:"+rule.ID] = base
				status := base
				status.Key += ".status"
				status.Values = []string{`"pass"`, `"fail"`}
				catalog.Add(status)
				for _, props := range []map[string]string{rule.OnSuccess, rule.OnFailure} {
					keys := make([]string, 0, len(props))
					for key := range props {
						keys = append(keys, key)
					}
					sort.Strings(keys)
					for _, key := range keys {
						v := base
						v.Key += "." + key
						raw, _ := json.Marshal(props[key])
						v.Values = []string{string(raw)}
						catalog.Add(v)
					}
				}
			case step.Collection != nil && step.Collection.Enabled && step.Collection.OutputKey != "":
				m := step.Collection
				base.Source = "collection"
				base.Key = m.OutputKey
				base.ProducerID = m.ID
				base.Type = "object"
				catalog.Add(base)
				producers["collection:"+m.ID] = base
				prefix := base.Key
				if m.Operation == models.ColOpFindMany {
					base.Type = "array"
					prefix += ".0"
				}
				status := base
				status.Key = prefix + "._status"
				status.Type = "string"
				status.Values = []string{`"success"`, `"not_found"`, `"error"`}
				catalog.Add(status)
				if h.collectionBackend != nil {
					docs, err := h.collectionBackend.GetAll(m.CollectionName)
					if err == nil {
						for i, doc := range docs {
							if i >= 50 {
								break
							}
							fields := base
							fields.Key = prefix
							fields.Origin = "collection"
							catalog.Observe(doc, fields, 0)
						}
					}
				}

			}
		}
	}
	steps, _ := h.buildPipelineSteps(func() ([]*models.ScriptBinding, error) { return h.store.GetSpecScriptBindings(specID) }, func() ([]*models.ValidationRule, error) { return h.store.ListValidationRulesBySpec(specID) }, func() ([]*models.CollectionMapping, error) { return h.store.GetCollectionMappingsBySpec(specID) })
	addSteps(steps, "spec")
	if len(ops) == 1 {
		id := ops[0].ID
		if status, err := strconv.Atoi(c.Query("statusCode")); err == nil {
			defs, err := h.parser.ExtractAllResponses(spec.Content, ops[0].Method, ops[0].Path)
			if err == nil {
				for _, def := range defs {
					if def.StatusCode == status && (c.Query("templateRef") == "" || def.ExampleName == c.Query("templateRef")) {
						catalog.Schema(def.Schema, mappinghints.Item{Source: "target", Origin: "schema"}, map[*openapi3.Schema]bool{}, 0)
						var example any
						if json.Unmarshal([]byte(def.BodyExample), &example) == nil {
							catalog.Observe(example, mappinghints.Item{Source: "target", Origin: "example"}, 0)
						}
						break
					}
				}
			}
		}
		responseID := c.Query("responseId")
		if responseID != "" {
			response, err := h.store.GetResponseConfig(responseID)
			if err != nil || response.OperationID != id {
				c.JSON(400, gin.H{"error": "response does not belong to operation"})
				return
			}
		}

		steps, _ = h.buildPipelineSteps(func() ([]*models.ScriptBinding, error) { return h.store.GetScriptBindings(id) }, func() ([]*models.ValidationRule, error) { return h.store.ListValidationRulesByOperation(id) }, func() ([]*models.CollectionMapping, error) { return h.store.GetCollectionMappingsByOperation(id) })
		addSteps(steps, "operation")
		if h.tracingService != nil {
			traces := h.tracingService.GetTraces(&models.TraceFilter{OperationID: id})
			if traceID := c.Query("traceId"); traceID != "" {
				trace := h.tracingService.GetTrace(traceID)
				if trace == nil || trace.OperationID != id {
					c.JSON(400, gin.H{"error": "trace not found for operation"})
					return
				}
				traces = []*models.Trace{trace}
			}
			count := 0
			for _, trace := range traces {
				if sessionID := c.Query("sessionId"); sessionID != "" && (trace.Session == nil || trace.Session.ID != sessionID) {
					continue
				}
				if count >= 50 {
					catalog.Truncated = true
					break
				}
				count++
				pattern := strings.Split(strings.Trim(ops[0].FullPath, "/"), "/")
				if ops[0].FullPath == "" {
					pattern = strings.Split(strings.Trim(ops[0].Path, "/"), "/")
				}
				actual := strings.Split(strings.Trim(trace.Request.Path, "/"), "/")
				if len(pattern) == len(actual) {
					for i, part := range pattern {
						if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
							catalog.Observe(actual[i], mappinghints.Item{Source: "path", Key: strings.Trim(part, "{}"), Origin: "observed"}, 0)
						}
					}
				}
				for key, values := range trace.Request.Query {
					if len(values) > 0 {
						catalog.Observe(values[0], mappinghints.Item{Source: "query", Key: key, Origin: "observed"}, 0)
					}
				}
				for key, values := range trace.Request.Headers {
					if len(values) > 0 {
						catalog.Observe(values[0], mappinghints.Item{Source: "header", Key: key, Origin: "observed"}, 0)
					}
				}
				var body any
				if json.Unmarshal([]byte(trace.Request.Body), &body) == nil {
					catalog.Observe(body, mappinghints.Item{Source: "body", Origin: "observed"}, 0)
				}
				if responseID != "" && trace.MatchedConfigID == responseID && trace.CollectionResponseRender != nil {
					render := trace.CollectionResponseRender
					if render.PrimaryMapper != nil && render.PrimaryMapper.Error == "" {
						catalog.Observe(render.PrimaryMapper.Result, mappinghints.Item{Source: "primary", Origin: "observed"}, 0)
					}
					for _, mapper := range render.AdditionalMappers {
						if mapper.Error == "" {
							catalog.Observe(mapper.Result, mappinghints.Item{Source: "mapper", Key: mapper.OutputKey, Origin: "observed"}, 0)
						}
					}
				}
				for _, output := range trace.Scripts {
					if base, ok := producers["script:"+output.BindingID]; ok && output.Error == "" {
						base.Origin = "observed"
						catalog.Observe(output.Output, base, 0)
					}
				}
				for _, output := range trace.Collections {
					if base, ok := producers["collection:"+output.MappingID]; ok && output.Error == "" {
						base.Origin = "observed"
						catalog.Observe(output.Result, base, 0)
					}
				}
			}
		}
	}
	var session store.SessionState
	if id := c.Query("sessionId"); id != "" {
		if h.sessionManager == nil {
			c.JSON(400, gin.H{"error": "sessions unavailable"})
			return
		}
		var ok bool
		session, ok, err = h.sessionManager.Get(id)
		if err != nil || !ok {
			c.JSON(404, gin.H{"error": "session not found"})
			return
		}
		for _, key := range session.Keys() {
			if !strings.HasPrefix(key, store.CollectionEventKeyPrefix) {
				catalog.Add(mappinghints.Item{Source: "session", Key: key, Type: "unknown", Origin: "session"})
			}
		}
	}
	if h.globalStore != nil {
		for _, entry := range h.globalStore.List() {
			catalog.Add(mappinghints.Item{Source: "store", Key: entry.Key, Type: mappinghints.JSONType(entry.Value), Origin: "store"})
		}
	}
	if name := c.Query("collectionName"); name != "" && h.collectionBackend != nil {
		// The backend currently exposes GetAll; only a bounded sample is inspected/returned.
		docs, err := h.collectionBackend.GetAll(name)
		if err != nil {
			catalog.Warnings = append(catalog.Warnings, "Collection fields unavailable")
		} else {
			if session != nil {
				docs = store.ReplayEvents(docs, store.LoadEvents(session, name))
			}
			for i, document := range docs {
				if i >= 50 {
					catalog.Truncated = true
					break
				}
				catalog.Observe(document, mappinghints.Item{Source: "document", Collection: name, Origin: "collection"}, 0)
			}
		}
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, catalog)
}

func addRequestHints(c *mappinghints.Catalog, doc *openapi3.T, op *models.Operation) {
	path := doc.Paths.Find(op.Path)
	if path == nil {
		return
	}
	operation := path.GetOperation(strings.ToUpper(op.Method))
	if operation == nil {
		return
	}
	for _, part := range strings.Split(op.Path, "/") {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			c.Add(mappinghints.Item{Source: "path", Key: strings.Trim(part, "{}"), Type: "string", Required: true, Origin: "schema"})
		}
	}
	params := map[string]*openapi3.Parameter{}
	for _, refs := range []openapi3.Parameters{path.Parameters, operation.Parameters} {
		for _, ref := range refs {
			if ref != nil && ref.Value != nil {
				p := ref.Value
				key := p.In + ":" + p.Name
				if p.In == "header" {
					key = strings.ToLower(key)
				}
				params[key] = p
			}
		}
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		p := params[k]
		if p.In != "path" && p.In != "query" && p.In != "header" {
			continue
		}
		base := mappinghints.Item{Source: p.In, Key: p.Name, Required: p.Required, Origin: "schema", Description: p.Description}
		if p.Schema != nil {
			parameterHints := mappinghints.New()
			parameterHints.Schema(p.Schema.Value, base, map[*openapi3.Schema]bool{}, 0)
			for _, hint := range parameterHints.Items {
				if hint.Key == p.Name {
					if hint.Description == "" {
						hint.Description = p.Description
					}
					c.Add(hint)
				}
			}
		} else {
			base.Type = "string"
			c.Add(base)
		}
	}
	if operation.RequestBody == nil || operation.RequestBody.Value == nil {
		return
	}
	body := operation.RequestBody.Value
	for contentType, media := range body.Content {
		if !strings.Contains(contentType, "json") || media == nil {
			continue
		}
		if media.Schema != nil {
			c.Schema(media.Schema.Value, mappinghints.Item{Source: "body", Required: body.Required}, map[*openapi3.Schema]bool{}, 0)
		}
		if media.Example != nil {
			c.Observe(media.Example, mappinghints.Item{Source: "body", Origin: "example"}, 0)
		}
		for _, example := range media.Examples {
			if example != nil && example.Value != nil {
				c.Observe(example.Value.Value, mappinghints.Item{Source: "body", Origin: "example"}, 0)
			}
		}
	}
}
