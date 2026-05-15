package models

// Condition represents a condition for matching requests
type Condition struct {
	Source   string `json:"source"`           // path, query, header, body
	Key      string `json:"key"`              // Parameter name or JSONPath for body
	Operator string `json:"operator"`          // eq, contains, regex, exists, gt, lt, gte, lte, dateEq, ...
	Value    string `json:"value"`             // Expected value (can be template or date token)
	Format   string `json:"format,omitempty"` // Optional Go time layout hint for date operators (e.g. "2006-01-02")
	Negate   bool   `json:"negate,omitempty"` // When true, inverts the operator result (replaces ne/notContains/notExists)
}

// Supported condition sources
const (
	SourcePath      = "path"
	SourceQuery     = "query"
	SourceHeader    = "header"
	SourceBody      = "body"
	SourceSignature = "signature" // Matches against a pre-computed request signature hash
)

// Supported condition operators
const (
	OpEquals      = "eq"
	OpContains    = "contains"
	OpRegex       = "regex"
	OpExists      = "exists"
	OpGreaterThan = "gt"
	OpLessThan    = "lt"
	OpGTE         = "gte"
	OpLTE         = "lte"
	OpStartsWith  = "startsWith"
	OpEndsWith    = "endsWith"

	// Date operators — the extracted value is parsed as a date/datetime.
	// The Value field accepts date literals or dynamic tokens:
	//   now, today, yesterday, tomorrow,
	//   now+Nd / now-Nd (days), now+Nh / now-Nh (hours), now+Nm / now-Nm (minutes).
	// The optional Format field sets the Go time layout used to parse the extracted value;
	// when absent, common layouts are tried in order.
	OpDateEquals   = "dateEq"
	OpDateBefore   = "dateBefore"
	OpDateAfter    = "dateAfter"
	OpDateLte      = "dateLte"
	OpDateGte      = "dateGte"
	OpDateInPast   = "dateInPast"   // no Value needed — true when extracted date < now
	OpDateInFuture = "dateInFuture" // no Value needed — true when extracted date > now
	OpDateToday    = "dateToday"    // no Value needed — true when extracted date is today (UTC)
	OpDateBetween  = "dateBetween"  // Value = "<from>,<to>" (tokens or literals)

	// Deprecated operators — still evaluated for backward compatibility with stored
	// conditions, but superseded by Condition.Negate = true on the positive variant.
	// Do not use these in new conditions; they are excluded from ValidOperators().
	//
	// Deprecated: use eq + Negate:true
	OpNotEquals = "ne"
	// Deprecated: use contains + Negate:true
	OpNotContains = "notContains"
	// Deprecated: use exists + Negate:true
	OpNotExists = "notExists"
)

// ValidSources returns all valid condition sources
func ValidSources() []string {
	return []string{SourcePath, SourceQuery, SourceHeader, SourceBody, SourceSignature}
}

// ValidOperators returns all current (non-deprecated) condition operators.
func ValidOperators() []string {
	return []string{
		OpEquals, OpContains,
		OpRegex, OpExists, OpGreaterThan,
		OpLessThan, OpGTE, OpLTE, OpStartsWith, OpEndsWith,
		OpDateEquals, OpDateBefore, OpDateAfter, OpDateLte, OpDateGte,
		OpDateInPast, OpDateInFuture, OpDateToday, OpDateBetween,
	}
}

// DeprecatedOperators returns operators that still evaluate correctly but are
// superseded by Condition.Negate on the positive equivalent.
func DeprecatedOperators() []string {
	return []string{OpNotEquals, OpNotContains, OpNotExists}
}

// NormaliseDeprecatedOperator converts a condition using a deprecated negative
// operator into its positive equivalent with Negate set to true. Conditions that
// already use current operators are returned unchanged.
func NormaliseDeprecatedOperator(c Condition) Condition {
	switch c.Operator {
	case OpNotEquals:
		c.Operator = OpEquals
		c.Negate = !c.Negate // preserve any existing negate
	case OpNotContains:
		c.Operator = OpContains
		c.Negate = !c.Negate
	case OpNotExists:
		c.Operator = OpExists
		c.Negate = !c.Negate
	}
	return c
}
