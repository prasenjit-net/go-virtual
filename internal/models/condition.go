package models

// Condition represents a condition for matching requests
type Condition struct {
	Source   string `json:"source"`           // path, query, header, body
	Key      string `json:"key"`              // Parameter name or JSONPath for body
	Operator string `json:"operator"`          // eq, ne, contains, regex, exists, notExists, gt, lt, gte, lte, dateEq, ...
	Value    string `json:"value"`             // Expected value (can be template or date token)
	Format   string `json:"format,omitempty"` // Optional Go time layout hint for date operators (e.g. "2006-01-02")
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
	OpNotEquals   = "ne"
	OpContains    = "contains"
	OpNotContains = "notContains"
	OpRegex       = "regex"
	OpExists      = "exists"
	OpNotExists   = "notExists"
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
	OpDateEquals    = "dateEq"
	OpDateBefore    = "dateBefore"
	OpDateAfter     = "dateAfter"
	OpDateLte       = "dateLte"
	OpDateGte       = "dateGte"
	OpDateInPast    = "dateInPast"    // no Value needed — true when extracted date < now
	OpDateInFuture  = "dateInFuture"  // no Value needed — true when extracted date > now
	OpDateToday     = "dateToday"     // no Value needed — true when extracted date is today (UTC)
	OpDateBetween   = "dateBetween"   // Value = "<from>,<to>" (tokens or literals)
)

// ValidSources returns all valid condition sources
func ValidSources() []string {
	return []string{SourcePath, SourceQuery, SourceHeader, SourceBody, SourceSignature}
}

// ValidOperators returns all valid condition operators
func ValidOperators() []string {
	return []string{
		OpEquals, OpNotEquals, OpContains, OpNotContains,
		OpRegex, OpExists, OpNotExists, OpGreaterThan,
		OpLessThan, OpGTE, OpLTE, OpStartsWith, OpEndsWith,
		OpDateEquals, OpDateBefore, OpDateAfter, OpDateLte, OpDateGte,
		OpDateInPast, OpDateInFuture, OpDateToday, OpDateBetween,
	}
}
