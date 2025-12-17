package ticket

import "context"

type ValidatorManager struct {
	validators []TicketValidator
}

func NewValidatorManager() *ValidatorManager {
	return &ValidatorManager{
		validators: make([]TicketValidator, 0),
	}
}

func (m *ValidatorManager) AddValidator(v TicketValidator) {
	m.validators = append(m.validators, v)
}

func (m *ValidatorManager) HasValidators() bool {
	return len(m.validators) > 0
}

type TicketValidationResult struct {
	AnyValid   bool
	Results    map[string]ValidationResult
	AllMissing bool
}

func (m *ValidatorManager) ValidateMessage(ctx context.Context, message string) TicketValidationResult {
	if len(m.validators) == 0 {
		return TicketValidationResult{AnyValid: true}
	}

	results := make(map[string]ValidationResult)
	anyFound := false
	anyValid := false

	for _, validator := range m.validators {
		if validator.ContainsTicket(message) {
			ticket := validator.ExtractTicket(message)
			result := validator.ValidateTicket(ctx, ticket)
			results[validator.Name()] = result
			anyFound = true
			if result.Valid {
				anyValid = true
			}
		}
	}

	return TicketValidationResult{
		AnyValid:   anyValid,
		Results:    results,
		AllMissing: !anyFound,
	}
}
