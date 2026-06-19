package validation

import (
	"time"

	"github.com/prasenjit/go-virtual/internal/condition"
	"github.com/prasenjit/go-virtual/internal/models"
)

// RunRule evaluates a single enabled validation rule and returns its result and trace.
// The caller is responsible for checking rule.Enabled before calling.
func RunRule(rule *models.ValidationRule, data *condition.RequestData) (*models.ValidationResult, models.ValidationTrace) {
	eval := condition.NewEvaluator()
	start := time.Now()
	passed := eval.EvaluateTree(rule.ConditionTree, data)
	durationMs := time.Since(start).Milliseconds()

	status := "pass"
	props := rule.OnSuccess
	if !passed {
		status = "fail"
		props = rule.OnFailure
	}

	scope := "operation"
	if rule.SpecID != "" {
		scope = "spec"
	}
	return &models.ValidationResult{Status: status, Properties: props},
		models.ValidationTrace{
			RuleID:     rule.ID,
			RuleName:   rule.Name,
			Scope:      scope,
			Status:     status,
			Properties: props,
			DurationMs: durationMs,
		}
}

// RunRules evaluates all enabled rules in order and returns the named output map
// and a list of trace entries.
// The returned output map is safe to assign to RequestData.ValidationOutput.
func RunRules(rules []*models.ValidationRule, data *condition.RequestData) (map[string]*models.ValidationResult, []models.ValidationTrace) {
	output := make(map[string]*models.ValidationResult)
	var traces []models.ValidationTrace

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		result, trace := RunRule(rule, data)
		output[rule.Name] = result
		traces = append(traces, trace)
	}
	return output, traces
}
