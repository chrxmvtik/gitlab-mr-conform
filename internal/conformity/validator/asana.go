package validator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"
	"gitlab-mr-conformity-bot/internal/config"
)

var (
	asanaPrefixRegex = regexp.MustCompile(`\b([A-Z][A-Z0-9]*)-(\d{16})\b`)
	asanaURLRegex    = regexp.MustCompile(`https://app\.asana\.com/\d+/\d+/(?:project/)?(?:\d+/)?(?:task/)?(\d{16})`)
)

type AsanaValidator struct {
	config     config.AsanaValidatorConfig
	httpClient *http.Client
	baseURL    string
}

func NewAsanaValidator(cfg config.AsanaValidatorConfig) *AsanaValidator {
	return &AsanaValidator{
		config:  cfg,
		baseURL: "https://app.asana.com",
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (v *AsanaValidator) Name() string {
	return "Asana"
}

func (v *AsanaValidator) ContainsTicket(message string) bool {
	return asanaPrefixRegex.MatchString(message) || asanaURLRegex.MatchString(message)
}

func (v *AsanaValidator) ExtractTicket(message string) *TicketInfo {
	if matches := asanaPrefixRegex.FindStringSubmatch(message); len(matches) >= 3 {
		return &TicketInfo{
			ProjectKey: matches[1],
			TicketID:   matches[2],
			FullMatch:  matches[0],
		}
	}

	if matches := asanaURLRegex.FindStringSubmatch(message); len(matches) >= 2 {
		return &TicketInfo{
			ProjectKey: "",
			TicketID:   matches[1],
			FullMatch:  matches[0],
		}
	}

	return nil
}

func (v *AsanaValidator) ValidateTicket(ctx context.Context, info *TicketInfo) ValidationResult {
	if info == nil {
		return ValidationResult{Found: false, Valid: false}
	}

	if info.ProjectKey != "" {
		validPrefix := false
		for _, key := range v.config.Keys {
			if key == info.ProjectKey {
				validPrefix = true
				break
			}
		}
		if !validPrefix {
			return ValidationResult{
				Found:      true,
				Valid:      false,
				TicketInfo: info,
				Error:      "Invalid Asana project prefix",
			}
		}
	}

	if v.config.ValidateExistence && v.config.APIToken != "" {
		exists, err := v.checkTaskExists(ctx, info.TicketID)
		if err != nil {
			return ValidationResult{
				Found:      true,
				Valid:      false,
				TicketInfo: info,
				Error:      fmt.Sprintf("API validation failed: %v", err),
			}
		}
		if !exists {
			return ValidationResult{
				Found:      true,
				Valid:      false,
				TicketInfo: info,
				Error:      "Asana task does not exist",
			}
		}
	}

	return ValidationResult{
		Found:      true,
		Valid:      true,
		TicketInfo: info,
	}
}

func (v *AsanaValidator) checkTaskExists(ctx context.Context, taskID string) (bool, error) {
	url := fmt.Sprintf("%s/api/1.0/tasks/%s", v.baseURL, taskID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("Authorization", "Bearer "+v.config.APIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	return true, nil
}
