package models

// CollectionOpType identifies the operation a CollectionMapping performs.
type CollectionOpType string

const (
	ColOpInsert   CollectionOpType = "insert"
	ColOpUpdate   CollectionOpType = "update"
	ColOpUpsert   CollectionOpType = "upsert"
	ColOpDelete   CollectionOpType = "delete"
	ColOpFindOne  CollectionOpType = "find-one"
	ColOpFindMany CollectionOpType = "find-many"
)

// FieldMappingRule maps one collection document field to a value sourced from
// the incoming request.
type FieldMappingRule struct {
	// TargetField is the document field name in the collection.
	TargetField string `json:"targetField"`
	// SourceType determines where the value is read from:
	// path | query | header | body | session | store | literal
	SourceType string `json:"sourceType"`
	// SourceKey is interpreted according to SourceType:
	//   path/query/header/session/store → parameter or key name
	//   body → dot-notation JSON path (e.g. "user.email")
	//   literal → the value itself
	SourceKey string `json:"sourceKey"`
}

// CollectionMapping attaches collection read/write operations to a Spec,
// Operation, or ResponseConfig. Exactly one of SpecID, OperationID, or
// ResponseConfigID is non-empty. Results are exposed in templates as
// {{.Collection.<OutputKey>.<field>}}.
type CollectionMapping struct {
	ID string `json:"id"`
	// Scope — exactly one is set.
	SpecID           string `json:"specId,omitempty"`
	OperationID      string `json:"operationId,omitempty"`
	ResponseConfigID string `json:"responseConfigId,omitempty"`

	CollectionName string           `json:"collectionName"`
	Name           string           `json:"name"`
	Operation      CollectionOpType `json:"operation"`
	// FilterRules locate the target record(s) — used by find-one, find-many,
	// update, upsert, delete.
	FilterRules []FieldMappingRule `json:"filterRules"`
	// DataRules specify the fields to write — used by insert, update, upsert.
	DataRules []FieldMappingRule `json:"dataRules"`
	// OutputKey names the key under which the result is available in templates.
	OutputKey string `json:"outputKey"`
	Order     int    `json:"order"`
	Enabled   bool   `json:"enabled"`
}

// Scope returns a string identifying which scope this mapping is attached to.
func (m *CollectionMapping) Scope() string {
	if m.SpecID != "" {
		return "spec"
	}
	if m.OperationID != "" {
		return "operation"
	}
	return "response"
}

// CollectionMappingInput is used for create/update API calls.
type CollectionMappingInput struct {
	CollectionName string             `json:"collectionName"`
	Name           string             `json:"name"`
	Operation      CollectionOpType   `json:"operation"`
	FilterRules    []FieldMappingRule `json:"filterRules"`
	DataRules      []FieldMappingRule `json:"dataRules"`
	OutputKey      string             `json:"outputKey"`
	Order          int                `json:"order"`
	Enabled        bool               `json:"enabled"`
}

// CollectionTrace captures execution of one CollectionMapping within a request.
type CollectionTrace struct {
	MappingID      string           `json:"mappingId"`
	MappingName    string           `json:"mappingName"`
	CollectionName string           `json:"collectionName"`
	Operation      CollectionOpType `json:"operation"`
	OutputKey      string           `json:"outputKey"`
	DurationMs     float64          `json:"durationMs"`
	RecordCount    int              `json:"recordCount"`
	Error          string           `json:"error,omitempty"`
}
