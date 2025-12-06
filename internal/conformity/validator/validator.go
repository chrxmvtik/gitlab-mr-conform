package validator

import "context"

type TicketInfo struct {
	ProjectKey string
	TicketID   string
	FullMatch  string
}

type ValidationResult struct {
	Found      bool
	Valid      bool
	TicketInfo *TicketInfo
	Error      string
}

type TicketValidator interface {
	Name() string
	ContainsTicket(message string) bool
	ExtractTicket(message string) *TicketInfo
	ValidateTicket(ctx context.Context, info *TicketInfo) ValidationResult
}
