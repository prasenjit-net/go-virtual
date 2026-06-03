package main

import (
	"log/slog"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/storage"
)

// seedDefaultAIScenarios creates the three built-in AI scenarios (success,
// client_error, server_error) if they do not already exist in the store.
// Existing scenarios are left untouched so user edits are preserved.
func seedDefaultAIScenarios(store storage.Storage, log *slog.Logger) {
	existing, err := store.ListAIScenarios()
	if err != nil {
		log.Warn("Setup: could not list AI scenarios — skipping seed",
			"event", "ai_scenario_seed_skipped", "error", err)
		return
	}

	existingByName := make(map[string]struct{}, len(existing))
	for _, s := range existing {
		if s != nil {
			existingByName[s.Name] = struct{}{}
		}
	}

	created := 0
	for _, s := range models.DefaultAIScenarios() {
		if _, ok := existingByName[s.Name]; ok {
			continue
		}
		sc := s
		if err := store.CreateAIScenario(&sc); err != nil {
			log.Warn("Setup: failed to create default AI scenario",
				"event", "ai_scenario_seed_failed", "name", s.Name, "error", err)
			continue
		}
		created++
	}

	if created > 0 {
		log.Info("Setup: seeded default AI scenarios",
			"event", "ai_scenario_seed_done", "created", created)
	} else {
		log.Info("Setup: default AI scenarios already present",
			"event", "ai_scenario_seed_skipped")
	}
}
