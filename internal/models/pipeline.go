package models

// PipelineStepType identifies the kind of component in a processing pipeline step.
type PipelineStepType string

const (
	PipelineStepScript     PipelineStepType = "script"
	PipelineStepValidation PipelineStepType = "validation"
	PipelineStepCollection PipelineStepType = "collection"
)

// PipelineStep is the wire format returned by the pipeline API endpoints.
// Exactly one of Script, Validation, Collection is non-nil.
type PipelineStep struct {
	Type       PipelineStepType   `json:"type"`
	Order      int                `json:"order"`
	Script     *ScriptBinding     `json:"script,omitempty"`
	Validation *ValidationRule    `json:"validation,omitempty"`
	Collection *CollectionMapping `json:"collection,omitempty"`
}

// PipelineReorderItem is one element of the reorder request body.
type PipelineReorderItem struct {
	Type PipelineStepType `json:"type"`
	ID   string           `json:"id"`
}
