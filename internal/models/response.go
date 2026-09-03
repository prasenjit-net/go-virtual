package models

const (
	ResponseOriginManual = "manual"
	ResponseOriginProxy  = "proxy"
	ResponseOriginAI     = "ai"
)

// ResponseKind distinguishes how a configured response's body is produced.
type ResponseKind string

const (
	// ResponseKindManual renders Body as a Go template, as it always has.
	ResponseKindManual ResponseKind = "manual"
	// ResponseKindCollection resolves its body from a collection query at
	// match time; see CollectionResponseConfig.
	ResponseKindCollection ResponseKind = "collection"
)

// ResponseConfig represents a configured response for an operation
type ResponseConfig struct {
	ID            string            `json:"id"`
	OperationID   string            `json:"operationId"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Tag           string            `json:"tag"`
	Priority      int               `json:"priority"` // Lower = higher priority (0 is highest)
	Conditions    []Condition       `json:"conditions"`
	ConditionTree *ConditionNode    `json:"conditionTree,omitempty"` // Tree replaces Conditions when set
	StatusCode    int               `json:"statusCode"`
	Headers       map[string]string `json:"headers"` // Can contain template variables
	Body          string            `json:"body"`    // Can contain template variables
	Delay         int               `json:"delay"`   // Response delay in milliseconds
	Enabled       bool              `json:"enabled"`
	Recorded      bool              `json:"recorded"` // True if auto-recorded in proxy mode
	Origin        string            `json:"origin"`

	// Kind selects how the body is produced. Empty normalizes to
	// ResponseKindManual — see EffectiveKind.
	Kind ResponseKind `json:"kind,omitempty"`
	// CollectionResponse holds the collection query, mappers, and field
	// overrides used when Kind is ResponseKindCollection. Nil for manual
	// responses.
	CollectionResponse *CollectionResponseConfig `json:"collectionResponse,omitempty"`
}

// EffectiveKind returns r.Kind, normalizing an empty/unrecognised value to
// ResponseKindManual so existing responses without a stored kind behave as
// before.
func (r *ResponseConfig) EffectiveKind() ResponseKind {
	if r == nil {
		return ResponseKindManual
	}
	if r.Kind == ResponseKindCollection {
		return ResponseKindCollection
	}
	return ResponseKindManual
}

// IsCollectionResponse reports whether this response resolves its body from
// a collection query rather than rendering Body as a template.
func (r *ResponseConfig) IsCollectionResponse() bool {
	return r.EffectiveKind() == ResponseKindCollection
}

// ResponseConfigInput represents input for creating/updating a response config
type ResponseConfigInput struct {
	Name               string                     `json:"name"`
	Description        string                     `json:"description"`
	Tag                string                     `json:"tag"`
	Priority           int                        `json:"priority"`
	Conditions         []Condition                `json:"conditions"`
	ConditionTree      *ConditionNode             `json:"conditionTree,omitempty"`
	StatusCode         int                        `json:"statusCode"`
	Headers            map[string]string          `json:"headers"`
	Body               string                     `json:"body"`
	Delay              int                        `json:"delay"`
	Enabled            bool                       `json:"enabled"`
	Kind               ResponseKind               `json:"kind,omitempty"`
	CollectionResponse *CollectionResponseConfig  `json:"collectionResponse,omitempty"`
}

// ResponseConfigUpdate represents input for updating a response config
type ResponseConfigUpdate struct {
	Name          *string            `json:"name,omitempty"`
	Description   *string            `json:"description,omitempty"`
	Tag           *string            `json:"tag,omitempty"`
	Priority      *int               `json:"priority,omitempty"`
	Conditions    *[]Condition       `json:"conditions,omitempty"`
	ConditionTree *ConditionNode     `json:"conditionTree,omitempty"`
	StatusCode    *int               `json:"statusCode,omitempty"`
	Headers       *map[string]string `json:"headers,omitempty"`
	Body          *string            `json:"body,omitempty"`
	Delay         *int               `json:"delay,omitempty"`
	Enabled       *bool              `json:"enabled,omitempty"`
	// CollectionResponse, when non-nil, replaces the collection response
	// configuration. Kind is immutable after creation and is not part of
	// the update payload.
	CollectionResponse *CollectionResponseConfig `json:"collectionResponse,omitempty"`
}

func NormalizeResponseOrigin(origin string, recorded bool) string {
	switch origin {
	case ResponseOriginProxy, ResponseOriginAI:
		return origin
	case ResponseOriginManual:
		return origin
	default:
		if recorded {
			return ResponseOriginProxy
		}
		return ResponseOriginManual
	}
}

func (r *ResponseConfig) NormalizeOrigin() {
	if r == nil {
		return
	}
	wasRecorded := r.Recorded
	r.Origin = r.EffectiveOrigin()
	switch r.Origin {
	case ResponseOriginManual:
		r.Recorded = false
	case ResponseOriginProxy:
		r.Recorded = true
	case ResponseOriginAI:
		r.Recorded = wasRecorded
	}
}

func (r *ResponseConfig) EffectiveOrigin() string {
	if r == nil {
		return ResponseOriginManual
	}
	return NormalizeResponseOrigin(r.Origin, r.Recorded)
}
