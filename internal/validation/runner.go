package validation

import (
	"time"

	"github.com/prasenjit/go-virtual/internal/condition"
	"github.com/prasenjit/go-virtual/internal/models"
)

// RunRules evaluates all enabled rules in order and returns the named output map
// and a list of trace entries.
// The returned output map is safe to assign to RequestData.ValidationOutput.
func RunRules(rules []*models.ValidationRule, data *condition.RequestData) (map[string]*models.ValidationResult, []models.ValidationTrace) {
	eval := condition.NewEvaluator()
	output := make(map[string]*models.ValidationResult)
	var traces []models.ValidationTrace

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		start := time.Now()
		passed := eval.EvaluateTree(rule.ConditionTree, data)
		durationMs := time.Since(start).Milliseconds()

		status := "pass"
		props := rule.OnSuccess
		if !passed {
			status = "fail"
			props = rule.OnFailure
		}

		result := &models.ValidationResult{Status: status, Properties: props}
		output[rule.Name] = result

		scope := "operation"
		if rule.SpecID != "" {
			scope = "spec"
		}
		traces = append(traces, models.ValidationTrace{
			RuleID:     rule.ID,
			RuleName:   rule.Name,
			Scope:      scope,
			Status:     status,
			Properties: props,
			DurationMs: durationMs,
		})
	}
	return output, traces
}
