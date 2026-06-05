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

// CollectionMapping attaches collection read/write operations to a ResponseConfig.
// Multiple mappings on one response config run in Order order and may each target
// a different collection. Results are exposed in templates as
// {{.Collection.<OutputKey>.<field>}}.
type CollectionMapping struct {
	ID               string             `json:"id"`
	ResponseConfigID string             `json:"responseConfigId"`
	CollectionName   string             `json:"collectionName"`
	Name             string             `json:"name"`
	Operation        CollectionOpType   `json:"operation"`
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
