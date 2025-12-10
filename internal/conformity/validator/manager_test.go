package validator

import (
	"context"
	"gitlab-mr-conformity-bot/internal/config"
	"testing"
)

func TestValidatorManager_ORLogic(t *testing.T) {
	manager := NewValidatorManager()

	manager.AddValidator(NewJiraValidator(config.JiraConfig{Keys: []string{"PROJ"}}))
	manager.AddValidator(NewAsanaValidator(config.AsanaValidatorConfig{Keys: []string{"DESIGN"}}, ""))

	tests := []struct {
		name           string
		message        string
		expectAnyValid bool
		expectMissing  bool
	}{
		{"valid jira only", "feat: test PROJ-123", true, false},
		{"valid asana only", "feat: test DESIGN-1234567890123456", true, false},
		{"both valid", "feat: PROJ-123 DESIGN-1234567890123456", true, false},
		{"invalid jira, valid asana", "feat: INVALID-123 DESIGN-1234567890123456", true, false},
		{"valid jira, invalid asana", "feat: PROJ-123 WRONG-1234567890123456", true, false},
		{"no tickets", "feat: test", false, true},
		{"both invalid", "feat: INVALID-123 WRONG-1234567890123456", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.ValidateMessage(context.Background(), tt.message)
			if result.AnyValid != tt.expectAnyValid {
				t.Errorf("expected AnyValid=%v, got %v. Results: %+v", tt.expectAnyValid, result.AnyValid, result.Results)
			}
			if result.AllMissing != tt.expectMissing {
				t.Errorf("expected AllMissing=%v, got %v", tt.expectMissing, result.AllMissing)
			}
		})
	}
}

func TestValidatorManager_NoValidators(t *testing.T) {
	manager := NewValidatorManager()

	result := manager.ValidateMessage(context.Background(), "feat: test")
	if !result.AnyValid {
		t.Error("expected AnyValid=true when no validators configured")
	}
	if result.AllMissing {
		t.Error("expected AllMissing=false when no validators configured")
	}
}

func TestValidatorManager_HasValidators(t *testing.T) {
	manager := NewValidatorManager()

	if manager.HasValidators() {
		t.Error("expected HasValidators=false for empty manager")
	}

	manager.AddValidator(NewJiraValidator(config.JiraConfig{Keys: []string{"PROJ"}}))

	if !manager.HasValidators() {
		t.Error("expected HasValidators=true after adding validator")
	}
}

func TestValidatorManager_Results(t *testing.T) {
	manager := NewValidatorManager()

	manager.AddValidator(NewJiraValidator(config.JiraConfig{Keys: []string{"PROJ"}}))
	manager.AddValidator(NewAsanaValidator(config.AsanaValidatorConfig{Keys: []string{"DESIGN"}}, ""))

	message := "feat: PROJ-123 WRONG-1234567890123456"
	result := manager.ValidateMessage(context.Background(), message)

	if len(result.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(result.Results))
	}

	jiraResult, hasJira := result.Results["Jira"]
	if !hasJira {
		t.Error("expected Jira result")
	}
	if !jiraResult.Valid {
		t.Error("expected valid Jira ticket")
	}

	asanaResult, hasAsana := result.Results["Asana"]
	if !hasAsana {
		t.Error("expected Asana result")
	}
	if asanaResult.Valid {
		t.Error("expected invalid Asana ticket")
	}
}
