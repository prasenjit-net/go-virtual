package models

// SpecWorkspace is the complete editable configuration for one uploaded spec.
// The OpenAPI document and imported operation contract are included for display
// and concurrency checks; the initial designer does not rewrite that contract.
type SpecWorkspace struct {
	Revision        string                   `json:"revision"`
	Spec            *Spec                    `json:"spec"`
	SpecScripts     []*ScriptBinding         `json:"specScripts"`
	SpecValidations []*ValidationRule        `json:"specValidations"`
	SpecMappings    []*CollectionMapping     `json:"specMappings"`
	Operations      []SpecWorkspaceOperation `json:"operations"`
}

type SpecWorkspaceOperation struct {
	Operation   *Operation              `json:"operation"`
	Scripts     []*ScriptBinding        `json:"scripts"`
	Validations []*ValidationRule       `json:"validations"`
	Mappings    []*CollectionMapping    `json:"mappings"`
	Responses   []SpecWorkspaceResponse `json:"responses"`
}

type SpecWorkspaceResponse struct {
	Response *ResponseConfig      `json:"response"`
	Scripts  []*ScriptBinding     `json:"scripts"`
	Mappings []*CollectionMapping `json:"mappings"`
}
