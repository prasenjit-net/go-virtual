package storage

import (
	"fmt"

	"github.com/prasenjit/go-virtual/internal/models"
)

// applyWorkspaceRecords writes a complete workspace to an isolated store. File
// storage uses it to build and verify a replacement snapshot before swapping it
// into service; it is intentionally not called against a live store.
func applyWorkspaceRecords(s Storage, w *models.SpecWorkspace) error {
	if err := s.UpdateSpec(w.Spec); err != nil {
		return fmt.Errorf("update spec: %w", err)
	}
	for _, op := range w.Operations {
		if err := s.UpdateOperation(op.Operation); err != nil {
			return fmt.Errorf("update operation %s: %w", op.Operation.ID, err)
		}
	}

	if err := s.DeleteScriptBindingsBySpec(w.Spec.ID); err != nil {
		return err
	}
	if err := s.DeleteCollectionMappingsBySpec(w.Spec.ID); err != nil {
		return err
	}
	oldSpecRules, err := s.ListValidationRulesBySpec(w.Spec.ID)
	if err != nil {
		return err
	}
	for _, old := range oldSpecRules {
		if err := s.DeleteValidationRule(old.ID); err != nil {
			return err
		}
	}

	for _, op := range w.Operations {
		oldBindings, err := s.GetScriptBindings(op.Operation.ID)
		if err != nil {
			return err
		}
		for _, old := range oldBindings {
			if err := s.DeleteScriptBinding(old.ID); err != nil {
				return err
			}
		}
		oldMappings, err := s.GetCollectionMappingsByOperation(op.Operation.ID)
		if err != nil {
			return err
		}
		for _, old := range oldMappings {
			if err := s.DeleteCollectionMapping(old.ID); err != nil {
				return err
			}
		}
		oldRules, err := s.ListValidationRulesByOperation(op.Operation.ID)
		if err != nil {
			return err
		}
		for _, old := range oldRules {
			if err := s.DeleteValidationRule(old.ID); err != nil {
				return err
			}
		}
		oldResponses, err := s.GetResponseConfigsByOperation(op.Operation.ID)
		if err != nil {
			return err
		}
		for _, old := range oldResponses {
			if err := s.DeleteScriptBindingsByResponse(old.ID); err != nil {
				return err
			}
			if err := s.DeleteCollectionMappingsByResponse(old.ID); err != nil {
				return err
			}
			if err := s.DeleteResponseConfig(old.ID); err != nil {
				return err
			}
		}
	}

	for _, binding := range w.SpecScripts {
		if err := s.CreateScriptBinding(binding); err != nil {
			return fmt.Errorf("create spec script binding %s: %w", binding.ID, err)
		}
	}
	for _, mapping := range w.SpecMappings {
		if err := s.CreateCollectionMapping(mapping); err != nil {
			return fmt.Errorf("create spec mapping %s: %w", mapping.ID, err)
		}
	}
	for _, rule := range w.SpecValidations {
		if _, err := s.CreateValidationRule(rule); err != nil {
			return fmt.Errorf("create spec validation %s: %w", rule.ID, err)
		}
	}
	for _, op := range w.Operations {
		for _, binding := range op.Scripts {
			if err := s.CreateScriptBinding(binding); err != nil {
				return fmt.Errorf("create operation script binding %s: %w", binding.ID, err)
			}
		}
		for _, mapping := range op.Mappings {
			if err := s.CreateCollectionMapping(mapping); err != nil {
				return fmt.Errorf("create operation mapping %s: %w", mapping.ID, err)
			}
		}
		for _, rule := range op.Validations {
			if _, err := s.CreateValidationRule(rule); err != nil {
				return fmt.Errorf("create operation validation %s: %w", rule.ID, err)
			}
		}
		for _, response := range op.Responses {
			if err := s.CreateResponseConfig(response.Response); err != nil {
				return fmt.Errorf("create response %s: %w", response.Response.ID, err)
			}
			for _, binding := range response.Scripts {
				if err := s.CreateScriptBinding(binding); err != nil {
					return fmt.Errorf("create response script binding %s: %w", binding.ID, err)
				}
			}
			for _, mapping := range response.Mappings {
				if err := s.CreateCollectionMapping(mapping); err != nil {
					return fmt.Errorf("create response mapping %s: %w", mapping.ID, err)
				}
			}
		}
	}
	return nil
}
