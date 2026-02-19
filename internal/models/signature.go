package models

// SignatureConfig defines what parts of a request are used to compute
// the request signature for an operation. Signatures are used in proxy
// recording mode to deduplicate responses.
type SignatureConfig struct {
	// PathParams lists which path parameters to include.
	// Empty slice means include ALL path parameters (default behaviour).
	PathParams []string `json:"pathParams"`

	// QueryParams lists which query parameters to include.
	// Empty slice means include ALL query parameters (default behaviour).
	QueryParams []string `json:"queryParams"`

	// Headers lists specific request headers to include.
	// Empty slice means NO headers are included (default behaviour).
	Headers []string `json:"headers"`

	// IncludeBody controls whether the request body is part of the signature.
	// Defaults to true when SignatureConfig is nil.
	IncludeBody bool `json:"includeBody"`

	// BodyJsonPaths lists specific gjson paths to extract from the body.
	// When empty and IncludeBody is true the entire raw body is used.
	BodyJsonPaths []string `json:"bodyJsonPaths"`
}
