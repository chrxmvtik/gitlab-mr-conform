package codeowners

// postProcessValidation validates owners and handles special cases
func (p *Parser) postProcessValidation(codeowners *CODEOWNERSFile) {
	p.logger.Debug("PostProcessValidation", "defaultRulesCount", len(codeowners.DefaultRules), "sectionsCount", len(codeowners.Sections))

	// Validate default rules
	for i := range codeowners.DefaultRules {
		p.logger.Debug("Validation default rule", "ruleNum", i, "rule", codeowners.DefaultRules[i].Pattern)
		p.validateRule(&codeowners.DefaultRules[i])
		// Debug output after validation
		if len(codeowners.DefaultRules[i].Owners) > 0 {
			p.logger.Debug("Default rule owners after validation", "ruleNum", i, "rule", codeowners.DefaultRules[i].Pattern, "owners", codeowners.DefaultRules[i].Owners)
		}
	}

	// Validate section rules
	for i := range codeowners.Sections {
		p.logger.Debug("Starting validating section", "sectionNum", i, "name", codeowners.Sections[i].Name, "rulesCount", len(codeowners.Sections[i].Rules), "rules", codeowners.Sections[i].Rules)
		p.validateSection(&codeowners.Sections[i])
		for j := range codeowners.Sections[i].Rules {
			p.logger.Debug("Validating section", "sectionNum", i, "ruleNum", j, "name", codeowners.Sections[i].Name, "rule", codeowners.Sections[i].Rules[j])
			p.validateRule(&codeowners.Sections[i].Rules[j])
			// Debug output after validation
			if len(codeowners.Sections[i].Rules[j].Owners) > 0 {
				p.logger.Debug("Section rule owners after validation", "sectionNum", i, "name", codeowners.Sections[i].Name, "ruleNum", j, "rule", codeowners.Sections[i].Rules[j], "owners", codeowners.Sections[i].Rules[j].Owners)
			}
		}
	}
}

// validateRule validates a rule's owners
func (p *Parser) validateRule(rule *Rule) {
	p.logger.Debug("validateRule called for rule", "rule", rule.Pattern, "ownersCount", len(rule.Owners))
	if len(rule.Owners) > 0 {
		p.logger.Debug("Rule owners", "rule", rule.Pattern, "owners", rule.Owners)
	}
	if len(rule.Owners) == 0 {
		rule.HasZeroOwners = true
		// Don't mark as invalid here - this is handled at the section level
		return
	}

	validOwners := 0
	for i := range rule.Owners {
		owner := &rule.Owners[i]
		isAccessible := p.isOwnerAccessible(owner.Original)
		p.logger.Debug("Owner accessiblity result", "owner", owner.Original, "accessible", isAccessible)

		// IMPORTANT: Make sure to update the IsValid field
		owner.IsValid = isAccessible

		if isAccessible {
			validOwners++
			p.logger.Debug("Marked owner as valid", "owner", owner.Original)
		} else {
			p.logger.Debug("Marked owner as ivvalid", "owner", owner.Original)
		}
	}

	if validOwners == 0 {
		rule.HasZeroOwners = true
		rule.IsValid = false
	} else {
		rule.IsValid = true
	}

	p.logger.Debug("Rule validation complete", "rule", rule.Pattern, "validOwners", validOwners, "totalOwners", len(rule.Owners))
}

// validateSection validates a section's default owners
func (p *Parser) validateSection(section *Section) {
	for i := range section.DefaultOwners {
		owner := &section.DefaultOwners[i]
		if p.isOwnerAccessible(owner.Original) {
			owner.IsValid = true
		} else {
			owner.IsValid = false
		}
	}
}
