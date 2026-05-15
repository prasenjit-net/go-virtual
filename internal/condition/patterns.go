package condition

import (
	"regexp"
	"strings"
)

// RegexPatternEntry describes a named regex token exposed via the API.
type RegexPatternEntry struct {
	Token       string `json:"token"`
	Description string `json:"description"`
	Pattern     string `json:"pattern"`
}

// patternEntries is the ordered catalogue of named regex tokens.
// Patterns are anchored (^ … $) so they match the full extracted value.
var patternEntries = []RegexPatternEntry{
	{
		Token:       "uuid",
		Description: "UUID v1–v5 (hex with dashes, case-insensitive)",
		Pattern:     `(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
	},
	{
		Token:       "uuid4",
		Description: "UUID v4 (random, case-insensitive)",
		Pattern:     `(?i)^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`,
	},
	{
		Token:       "email",
		Description: "Email address (RFC 5321 simplified)",
		Pattern:     `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`,
	},
	{
		Token:       "url",
		Description: "HTTP or HTTPS URL",
		Pattern:     `^https?://[^\s/$.?#].[^\s]*$`,
	},
	{
		Token:       "ipv4",
		Description: "IPv4 dotted-decimal address",
		Pattern:     `^((25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(25[0-5]|2[0-4]\d|[01]?\d\d?)$`,
	},
	{
		Token:       "ipv6",
		Description: "IPv6 address (full or compressed)",
		Pattern:     `^(([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}|::([0-9a-fA-F]{1,4}:){0,6}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:))$`,
	},
	{
		Token:       "ip",
		Description: "IPv4 or IPv6 address",
		Pattern:     `^(((25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(25[0-5]|2[0-4]\d|[01]?\d\d?)|(([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}|::([0-9a-fA-F]{1,4}:){0,6}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:))$`,
	},
	{
		Token:       "us-phone",
		Description: "US phone number (10 digits, optional +1, common separators)",
		Pattern:     `^(\+?1[\s.\-]?)?(\(?\d{3}\)?[\s.\-]?)\d{3}[\s.\-]?\d{4}$`,
	},
	{
		Token:       "us-zip",
		Description: "US ZIP code (5-digit or ZIP+4)",
		Pattern:     `^\d{5}(-\d{4})?$`,
	},
	{
		Token:       "ssn",
		Description: "US Social Security Number (###-##-####)",
		Pattern:     `^\d{3}-\d{2}-\d{4}$`,
	},
	{
		Token:       "date-iso",
		Description: "ISO 8601 date (YYYY-MM-DD)",
		Pattern:     `^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])$`,
	},
	{
		Token:       "datetime-iso",
		Description: "ISO 8601 datetime with optional timezone",
		Pattern:     `^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])[T ]\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:?\d{2})?$`,
	},
	{
		Token:       "time-hms",
		Description: "24-hour time (HH:MM:SS)",
		Pattern:     `^([01]\d|2[0-3]):[0-5]\d:[0-5]\d$`,
	},
	{
		Token:       "integer",
		Description: "Whole number (optional leading minus)",
		Pattern:     `^-?\d+$`,
	},
	{
		Token:       "decimal",
		Description: "Decimal number (optional minus, optional fractional part)",
		Pattern:     `^-?\d+(\.\d+)?$`,
	},
	{
		Token:       "alpha",
		Description: "Letters only (a-z, A-Z)",
		Pattern:     `^[a-zA-Z]+$`,
	},
	{
		Token:       "alphanumeric",
		Description: "Letters and digits only",
		Pattern:     `^[a-zA-Z0-9]+$`,
	},
	{
		Token:       "slug",
		Description: "URL slug — lowercase letters, digits, and hyphens",
		Pattern:     `^[a-z0-9]+(-[a-z0-9]+)*$`,
	},
	{
		Token:       "hex-color",
		Description: "CSS hex colour (#RGB or #RRGGBB)",
		Pattern:     `^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`,
	},
	{
		Token:       "base64",
		Description: "Standard base64-encoded string",
		Pattern:     `^[A-Za-z0-9+/]+={0,2}$`,
	},
	{
		Token:       "jwt",
		Description: "JSON Web Token (three base64url segments)",
		Pattern:     `^[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_]*$`,
	},
	{
		Token:       "credit-card",
		Description: "Credit card number (13–19 digits, no spaces)",
		Pattern:     `^\d{13,19}$`,
	},
	{
		Token:       "iban",
		Description: "IBAN (country code + check digits + BBAN)",
		Pattern:     `(?i)^[A-Z]{2}\d{2}[A-Z0-9]{1,30}$`,
	},
	{
		Token:       "semver",
		Description: "Semantic version (major.minor.patch with optional pre-release and build metadata)",
		Pattern:     `^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*)?(\+[0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*)?$`,
	},
}

// patternMap is the compiled lookup table keyed by lowercase token name.
var patternMap map[string]*regexp.Regexp

func init() {
	patternMap = make(map[string]*regexp.Regexp, len(patternEntries))
	for _, e := range patternEntries {
		patternMap[e.Token] = regexp.MustCompile(e.Pattern)
	}
}

// Expand returns the full regex pattern for a known token name (case-insensitive).
// If value is not a recognised token it is returned unchanged, allowing raw regex
// patterns to be used directly.
func Expand(value string) string {
	if re, ok := patternMap[strings.ToLower(value)]; ok {
		return re.String()
	}
	return value
}

// IsToken reports whether value (case-insensitive) is a known pattern token.
func IsToken(value string) bool {
	_, ok := patternMap[strings.ToLower(value)]
	return ok
}

// PatternCatalogue returns the full ordered list of pattern tokens for use by the API.
func PatternCatalogue() []RegexPatternEntry {
	result := make([]RegexPatternEntry, len(patternEntries))
	copy(result, patternEntries)
	return result
}
