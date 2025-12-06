package validator

import (
	"context"
	"gitlab-mr-conformity-bot/internal/config"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAsanaValidator_ContainsTicket(t *testing.T) {
	validator := NewAsanaValidator(config.AsanaValidatorConfig{})

	tests := []struct {
		name     string
		message  string
		expected bool
	}{
		{"prefix format", "feat: add feature DESIGN-1234567890123456", true},
		{"URL format", "fix: bug https://app.asana.com/0/123/456/1234567890123456", true},
		{"URL with project", "feat: https://app.asana.com/1/123/project/456/task/1234567890123456", true},
		{"no ticket", "docs: update readme", false},
		{"lowercase prefix", "design-1234567890123456", false},
		{"wrong digit count", "PROJ-12345", false},
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

func TestAsanaValidator_ExtractTicket_Prefix(t *testing.T) {
	validator := NewAsanaValidator(config.AsanaValidatorConfig{})

	tests := []struct {
		name           string
		message        string
		expectNil      bool
		expectProjKey  string
		expectTicketID string
	}{
		{"simple prefix", "feat: DESIGN-1234567890123456", false, "DESIGN", "1234567890123456"},
		{"prefix with spaces", "fix: bug PROJ-1111111111111111 fixed", false, "PROJ", "1111111111111111"},
		{"no ticket", "docs: update", true, "", ""},
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

func TestAsanaValidator_ExtractTicket_URL(t *testing.T) {
	validator := NewAsanaValidator(config.AsanaValidatorConfig{})

	tests := []struct {
		name           string
		message        string
		expectTicketID string
	}{
		{"basic URL", "https://app.asana.com/0/123/456/1234567890123456", "1234567890123456"},
		{"URL with project", "https://app.asana.com/1/123/project/456/task/7777777777777777", "7777777777777777"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := validator.ExtractTicket(tt.message)
			if ticket == nil {
				t.Fatal("expected ticket, got nil")
			}
			if ticket.ProjectKey != "" {
				t.Errorf("expected empty ProjectKey for URL, got %s", ticket.ProjectKey)
			}
			if ticket.TicketID != tt.expectTicketID {
				t.Errorf("expected TicketID %s, got %s", tt.expectTicketID, ticket.TicketID)
			}
		})
	}
}

func TestAsanaValidator_ValidateTicket_PrefixValidation(t *testing.T) {
	validator := NewAsanaValidator(config.AsanaValidatorConfig{
		Keys:              []string{"DESIGN", "MARKETING"},
		ValidateExistence: false,
	})

	tests := []struct {
		name        string
		projectKey  string
		ticketID    string
		expectValid bool
		expectError bool
	}{
		{"valid DESIGN key", "DESIGN", "1234567890123456", true, false},
		{"valid MARKETING key", "MARKETING", "1234567890123456", true, false},
		{"invalid key", "INVALID", "1234567890123456", false, true},
		{"URL format (no key)", "", "1234567890123456", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := &TicketInfo{
				ProjectKey: tt.projectKey,
				TicketID:   tt.ticketID,
			}
			result := validator.ValidateTicket(context.Background(), ticket)
			if result.Valid != tt.expectValid {
				t.Errorf("expected Valid=%v, got %v (error: %s)", tt.expectValid, result.Valid, result.Error)
			}
			if tt.expectError && result.Error == "" {
				t.Error("expected error, got none")
			}
		})
	}
}

func TestAsanaValidator_ValidateTicket_WithAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/1.0/tasks/1234567890123456" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": {"gid": "1234567890123456"}}`))
		} else if r.URL.Path == "/api/1.0/tasks/9999999999999999" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	validator := NewAsanaValidator(config.AsanaValidatorConfig{
		Keys:              []string{"DESIGN"},
		ValidateExistence: true,
		APIToken:          "test-token",
	})

	validator.httpClient = server.Client()
	validator.baseURL = server.URL

	tests := []struct {
		name        string
		taskID      string
		expectValid bool
		expectError bool
	}{
		{"existing task", "1234567890123456", true, false},
		{"non-existing task", "9999999999999999", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			exists, err := validator.checkTaskExists(ctx, tt.taskID)
			if tt.expectError {
				if err == nil && !exists {
					return
				}
				if err != nil {
					return
				}
				t.Error("expected error or task not found")
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if !exists {
					t.Error("expected task to exist")
				}
			}
		})
	}
}

func TestAsanaValidator_ValidateTicket_APIDisabled(t *testing.T) {
	validator := NewAsanaValidator(config.AsanaValidatorConfig{
		Keys:              []string{"DESIGN"},
		ValidateExistence: false,
		APIToken:          "",
	})

	ticket := &TicketInfo{
		ProjectKey: "DESIGN",
		TicketID:   "1234567890123456",
	}

	result := validator.ValidateTicket(context.Background(), ticket)
	if !result.Valid {
		t.Errorf("expected valid without API check, got error: %s", result.Error)
	}
}

func TestAsanaValidator_ValidateTicket_NilInfo(t *testing.T) {
	validator := NewAsanaValidator(config.AsanaValidatorConfig{Keys: []string{"DESIGN"}})
	result := validator.ValidateTicket(context.Background(), nil)
	if result.Found {
		t.Error("expected Found=false for nil ticket info")
	}
	if result.Valid {
		t.Error("expected Valid=false for nil ticket info")
	}
}
