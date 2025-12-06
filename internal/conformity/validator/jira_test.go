package validator

import (
	"context"
	"gitlab-mr-conformity-bot/internal/config"
	"testing"
)

func TestJiraValidator_ContainsTicket(t *testing.T) {
	validator := NewJiraValidator(config.JiraConfig{Keys: []string{"PROJ"}})

	tests := []struct {
		name     string
		message  string
		expected bool
	}{
		{"with brackets", "feat: add feature [PROJ-123]", true},
		{"without brackets", "fix: bug fix PROJ-456", true},
		{"no ticket", "docs: update readme", false},
		{"lowercase", "proj-123", false},
		{"in middle", "feat(api): add endpoint for PROJ-789 validation", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ContainsTicket(tt.message)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestJiraValidator_ExtractTicket(t *testing.T) {
	validator := NewJiraValidator(config.JiraConfig{Keys: []string{"PROJ"}})

	tests := []struct {
		name           string
		message        string
		expectNil      bool
		expectProjKey  string
		expectTicketID string
	}{
		{"with brackets", "feat: add feature [PROJ-123]", false, "PROJ", "123"},
		{"without brackets", "fix: bug PROJ-456", false, "PROJ", "456"},
		{"no ticket", "docs: update", true, "", ""},
		{"multiple digits", "feat: test PROJ-1234567", false, "PROJ", "1234567"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := validator.ExtractTicket(tt.message)
			if tt.expectNil {
				if ticket != nil {
					t.Errorf("expected nil, got %+v", ticket)
				}
				return
			}
			if ticket == nil {
				t.Fatal("expected ticket, got nil")
			}
			if ticket.ProjectKey != tt.expectProjKey {
				t.Errorf("expected ProjectKey %s, got %s", tt.expectProjKey, ticket.ProjectKey)
			}
			if ticket.TicketID != tt.expectTicketID {
				t.Errorf("expected TicketID %s, got %s", tt.expectTicketID, ticket.TicketID)
			}
		})
	}
}

func TestJiraValidator_ValidateTicket(t *testing.T) {
	validator := NewJiraValidator(config.JiraConfig{Keys: []string{"PROJ", "JIRA"}})

	tests := []struct {
		name        string
		projectKey  string
		ticketID    string
		expectValid bool
		expectError bool
	}{
		{"valid PROJ key", "PROJ", "123", true, false},
		{"valid JIRA key", "JIRA", "456", true, false},
		{"invalid key", "INVALID", "789", false, true},
		{"empty key", "", "123", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := &TicketInfo{
				ProjectKey: tt.projectKey,
				TicketID:   tt.ticketID,
			}
			result := validator.ValidateTicket(context.Background(), ticket)
			if result.Valid != tt.expectValid {
				t.Errorf("expected Valid=%v, got %v", tt.expectValid, result.Valid)
			}
			if tt.expectError && result.Error == "" {
				t.Error("expected error, got none")
			}
		})
	}
}

func TestJiraValidator_ValidateTicket_NilInfo(t *testing.T) {
	validator := NewJiraValidator(config.JiraConfig{Keys: []string{"PROJ"}})
	result := validator.ValidateTicket(context.Background(), nil)
	if result.Found {
		t.Error("expected Found=false for nil ticket info")
	}
	if result.Valid {
		t.Error("expected Valid=false for nil ticket info")
	}
}

func TestJiraValidator_Debug(t *testing.T) {
	validator := NewJiraValidator(config.JiraConfig{Keys: []string{"PROJ"}})
	message := "feat: PROJ-123 WRONG-1234567890123456"
	
	t.Logf("Message: %s", message)
	t.Logf("Contains: %v", validator.ContainsTicket(message))
	
	ticket := validator.ExtractTicket(message)
	if ticket == nil {
		t.Fatal("Expected ticket, got nil")
	}
	t.Logf("Extracted: ProjectKey=%s, TicketID=%s", ticket.ProjectKey, ticket.TicketID)
	
	result := validator.ValidateTicket(context.Background(), ticket)
	t.Logf("Valid: %v, Error: %s", result.Valid, result.Error)
	
	if !result.Valid {
		t.Errorf("Expected valid PROJ ticket")
	}
}
