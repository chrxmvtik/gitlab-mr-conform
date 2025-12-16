package ticket

import "gitlab-mr-conformity-bot/internal/config"

func BuildTicketValidators(jiraCfg config.JiraConfig, asanaCfg config.AsanaValidatorConfig, integrations config.IntegrationsConfig) *ValidatorManager {
	manager := NewValidatorManager()

	if len(jiraCfg.Keys) > 0 && jiraCfg.Keys[0] != "" {
		jiraValidator := NewJiraValidator(jiraCfg)
		manager.AddValidator(jiraValidator)
	}

	if len(asanaCfg.Keys) > 0 && asanaCfg.Keys[0] != "" {
		asanaValidator := NewAsanaValidator(asanaCfg, integrations.Asana.APIToken)
		manager.AddValidator(asanaValidator)
	}

	return manager
}
