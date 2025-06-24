package codeowners

import (
	"fmt"
)

// GetOwnersForFile returns the owners for a given file path with pattern matching details
func (c *CODEOWNERSFile) GetOwnersForFile(filePath string) map[string]SectionOwnership {
	result := make(map[string]SectionOwnership)

	// Process default rules
	owners, isAutoApproved, validationErrors, usedDefaultOwners, matchingPatterns := c.getMatchingOwnersWithPatternsAndValidation(c.DefaultRules, filePath, nil, "Default")
	if len(owners) > 0 || isAutoApproved || len(matchingPatterns) > 0 {
		result["Default"] = SectionOwnership{
			Name:              "Default",
			Owners:            owners,
			RequiredApprovals: 1,
			IsOptional:        false,
			IsAutoApproved:    isAutoApproved,
			ValidationErrors:  validationErrors,
			UsedDefaultOwners: usedDefaultOwners,
			MatchingPatterns:  matchingPatterns,
		}
	}

	// Process each section
	for _, section := range c.Sections {
		owners, isAutoApproved, validationErrors, usedDefaultOwners, matchingPatterns := c.getMatchingOwnersWithPatternsAndValidation(section.Rules, filePath, &section, section.Name)

		if len(owners) > 0 || isAutoApproved || len(matchingPatterns) > 0 {
			// Determine the correct required approvals
			requiredApprovals := 1 // Default for explicit owners
			if usedDefaultOwners {
				// Only use section's required approvals if we're using section default owners
				requiredApprovals = section.RequiredApprovals
			}

			result[section.Name] = SectionOwnership{
				Name:              section.Name,
				Owners:            owners,
				RequiredApprovals: requiredApprovals,
				IsOptional:        section.IsOptional,
				IsAutoApproved:    isAutoApproved,
				ValidationErrors:  validationErrors,
				UsedDefaultOwners: usedDefaultOwners,
				MatchingPatterns:  matchingPatterns,
			}
		}
	}

	return result
}

// getMatchingOwnersWithPatternsAndValidation finds owners with validation info and pattern tracking
func (c *CODEOWNERSFile) getMatchingOwnersWithPatternsAndValidation(rules []Rule, filePath string, section *Section, sectionName string) ([]Owner, bool, []string, bool, []MatchingPattern) {
	var matchedOwners []Owner
	var validationErrors []string
	var matchingPatterns []MatchingPattern
	excluded := false
	hasMatch := false
	isAutoApproved := false
	usedDefaultOwners := false
	activePatternIndex := -1

	// Ensure pattern matcher is initialized
	if c.patternMatcher == nil {
		c.patternMatcher = NewPatternMatcher()
	}

	// Process rules in order (later rules take precedence)
	for i, rule := range rules {
		if c.patternMatcher.MatchesPattern(rule.Pattern, filePath) {
			hasMatch = true

			// Get match type using the new method
			matchType := c.patternMatcher.GetMatchType(rule.Pattern, filePath)

			// Create matching pattern record
			matchingPattern := MatchingPattern{
				Pattern:     rule.Pattern,
				IsExclusion: rule.IsExclusion,
				LineNumber:  rule.LineNumber,
				RuleIndex:   i,
				MatchType:   matchType,
				IsActive:    false,
			}

			// If there was a previous active pattern, mark it as overridden
			if activePatternIndex >= 0 {
				matchingPatterns[activePatternIndex].IsActive = false
				matchingPatterns[activePatternIndex].OverriddenBy = &matchingPattern
			}

			matchingPatterns = append(matchingPatterns, matchingPattern)
			activePatternIndex = len(matchingPatterns) - 1

			if rule.IsExclusion {
				excluded = true
				matchedOwners = nil
				isAutoApproved = false
				usedDefaultOwners = false
				matchingPatterns[activePatternIndex].IsActive = true
			} else if !excluded {
				matchingPatterns[activePatternIndex].IsActive = true

				// FIXED: Check if rule has parsing errors first
				if rule.HasParseError && !rule.IsValid {
					// If there was a parsing error, auto-approve regardless of section default owners
					isAutoApproved = true
					matchedOwners = nil
					usedDefaultOwners = false
				} else if rule.HasZeroOwners {
					// Check if we have section default owners to use
					if section != nil && len(section.DefaultOwners) > 0 {
						// Use section default owners
						validOwners := make([]Owner, 0)
						for _, owner := range section.DefaultOwners {
							if owner.IsValid {
								validOwners = append(validOwners, owner)
							} else {
								validationErrors = append(validationErrors,
									fmt.Sprintf("inaccessible section default owner: %s", owner.Original))
							}
						}
						matchedOwners = validOwners
						isAutoApproved = len(validOwners) == 0
						usedDefaultOwners = true
					} else {
						// No section default owners, so it's auto-approved
						isAutoApproved = true
						matchedOwners = nil
						usedDefaultOwners = false
					}
				} else {
					// Rule has explicit owners
					validOwners := make([]Owner, 0)
					for _, owner := range rule.Owners {
						if owner.IsValid {
							validOwners = append(validOwners, owner)
						} else {
							validationErrors = append(validationErrors,
								fmt.Sprintf("inaccessible owner: %s", owner.Original))
						}
					}
					matchedOwners = validOwners
					isAutoApproved = len(validOwners) == 0
					usedDefaultOwners = false

					// Pass parse errors to validation errors
					validationErrors = append(validationErrors, rule.ParseError)

				}
			}
		}
	}

	if !hasMatch {
		return nil, false, nil, false, nil
	}

	return matchedOwners, isAutoApproved, validationErrors, usedDefaultOwners, matchingPatterns
}

// GetOwnersAsStrings returns all valid owners for a file
func (c *CODEOWNERSFile) GetOwnersAsStrings(filePath string) []string {
	ownersMap := c.GetOwnersForFile(filePath)
	var allOwners []string
	seen := make(map[string]bool)

	for _, sectionOwnership := range ownersMap {
		for _, owner := range sectionOwnership.Owners {
			if !owner.IsValid {
				continue // Skip invalid owners
			}

			ownerStr := ""
			switch owner.Type {
			case OwnerTypeUser:
				if owner.IsEmail {
					ownerStr = owner.Name
				} else {
					ownerStr = owner.Name
				}
			case OwnerTypeGroup:
				ownerStr = owner.Name
			case OwnerTypeRole:
				ownerStr = owner.Name
			}

			if ownerStr != "" && !seen[ownerStr] {
				allOwners = append(allOwners, ownerStr)
				seen[ownerStr] = true
			}
		}
	}

	return allOwners
}

// GetMatchingPatternsForFile returns detailed pattern matching information
func (c *CODEOWNERSFile) GetMatchingPatternsForFile(filePath string) map[string][]MatchingPattern {
	result := make(map[string][]MatchingPattern)

	ownersMap := c.GetOwnersForFile(filePath)
	for sectionName, ownership := range ownersMap {
		if len(ownership.MatchingPatterns) > 0 {
			result[sectionName] = ownership.MatchingPatterns
		}
	}

	return result
}

// GetActivePatternForFile returns the currently active pattern for a file in each section
func (c *CODEOWNERSFile) GetActivePatternForFile(filePath string) map[string]*MatchingPattern {
	result := make(map[string]*MatchingPattern)

	ownersMap := c.GetOwnersForFile(filePath)
	for sectionName, ownership := range ownersMap {
		for i := range ownership.MatchingPatterns {
			if ownership.MatchingPatterns[i].IsActive {
				result[sectionName] = &ownership.MatchingPatterns[i]
				break
			}
		}
	}

	return result
}

func (c *CODEOWNERSFile) PrecompilePatterns() error {
	if c.patternMatcher == nil {
		c.patternMatcher = NewPatternMatcher()
	}

	var patterns []string

	// Collect all patterns
	for _, rule := range c.DefaultRules {
		patterns = append(patterns, rule.Pattern)
	}

	for _, section := range c.Sections {
		for _, rule := range section.Rules {
			patterns = append(patterns, rule.Pattern)
		}
	}

	// Compile all patterns
	for _, pattern := range patterns {
		if err := c.patternMatcher.CompilePattern(pattern); err != nil {
			return fmt.Errorf("failed to compile pattern %s: %w", pattern, err)
		}
	}

	return nil
}

// Enhanced String method for MatchingPattern
func (mp MatchingPattern) String() string {
	prefix := ""
	if mp.IsExclusion {
		prefix = "!"
	}

	status := ""
	if mp.IsActive {
		status = " (active)"
	} else if mp.OverriddenBy != nil {
		status = fmt.Sprintf(" (overridden by %s%s)",
			func() string {
				if mp.OverriddenBy.IsExclusion {
					return "!"
				} else {
					return ""
				}
			}(),
			mp.OverriddenBy.Pattern)
	}

	return fmt.Sprintf("%s%s [line %d, %s match]%s",
		prefix, mp.Pattern, mp.LineNumber, mp.MatchType, status)
}
