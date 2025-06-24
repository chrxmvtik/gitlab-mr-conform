package codeowners

import (
	"fmt"
	"strings"
)

// GetValidationReport returns a comprehensive validation report
func (c *CODEOWNERSFile) GetValidationReport() map[string]interface{} {
	report := make(map[string]interface{})

	// Count statistics
	totalRules := len(c.DefaultRules)
	invalidRules := 0
	zeroOwnerRules := 0

	for _, section := range c.Sections {
		totalRules += len(section.Rules)
	}

	// Check default rules
	for _, rule := range c.DefaultRules {
		if !rule.IsValid {
			invalidRules++
		}
		if rule.HasZeroOwners {
			zeroOwnerRules++
		}
	}

	// Check section rules
	for _, section := range c.Sections {
		for _, rule := range section.Rules {
			if !rule.IsValid {
				invalidRules++
			}
			if rule.HasZeroOwners {
				zeroOwnerRules++
			}
		}
	}

	report["total_rules"] = totalRules
	report["invalid_rules"] = invalidRules
	report["zero_owner_rules"] = zeroOwnerRules
	report["total_sections"] = len(c.Sections)
	report["parse_errors"] = c.ParseErrors

	return report
}

// GetDetailedMatchReport returns a comprehensive report of pattern matching
func (c *CODEOWNERSFile) GetDetailedMatchReport(filePath string) map[string]interface{} {
	report := make(map[string]interface{})
	ownersMap := c.GetOwnersForFile(filePath)

	report["file_path"] = filePath
	report["sections"] = make(map[string]interface{})

	totalMatches := 0
	totalActiveMatches := 0

	for sectionName, ownership := range ownersMap {
		sectionReport := make(map[string]interface{})
		sectionReport["owners_count"] = len(ownership.Owners)
		sectionReport["required_approvals"] = ownership.RequiredApprovals
		sectionReport["is_optional"] = ownership.IsOptional
		sectionReport["is_auto_approved"] = ownership.IsAutoApproved
		sectionReport["used_default_owners"] = ownership.UsedDefaultOwners
		sectionReport["validation_errors"] = ownership.ValidationErrors

		patterns := make([]map[string]interface{}, 0)
		activePattern := ""

		for _, pattern := range ownership.MatchingPatterns {
			totalMatches++
			if pattern.IsActive {
				totalActiveMatches++
				activePattern = pattern.String()
			}

			patternInfo := map[string]interface{}{
				"pattern":      pattern.Pattern,
				"is_exclusion": pattern.IsExclusion,
				"line_number":  pattern.LineNumber,
				"rule_index":   pattern.RuleIndex,
				"match_type":   pattern.MatchType,
				"is_active":    pattern.IsActive,
			}

			if pattern.OverriddenBy != nil {
				patternInfo["overridden_by"] = pattern.OverriddenBy.Pattern
			}

			patterns = append(patterns, patternInfo)
		}

		sectionReport["matching_patterns"] = patterns
		sectionReport["active_pattern"] = activePattern
		sectionReport["pattern_count"] = len(ownership.MatchingPatterns)

		report["sections"].(map[string]interface{})[sectionName] = sectionReport
	}

	report["total_matches"] = totalMatches
	report["total_active_matches"] = totalActiveMatches

	return report
}

// String methods for pretty printing (enhanced)

func (o Owner) String() string {
	suffix := ""
	if !o.IsValid {
		suffix = " (inaccessible)"
	}

	switch o.Type {
	case OwnerTypeRole:
		return fmt.Sprintf("@@%s%s", o.Name, suffix)
	case OwnerTypeGroup:
		return fmt.Sprintf("@%s%s", o.Name, suffix)
	case OwnerTypeUser:
		if o.IsEmail {
			return o.Name + suffix
		}
		return fmt.Sprintf("@%s%s", o.Name, suffix)
	}
	return o.Name + suffix
}

func (r Rule) String() string {
	prefix := ""
	if r.IsExclusion {
		prefix = "!"
	}

	ownersStr := ""
	if len(r.Owners) > 0 {
		ownerStrs := make([]string, len(r.Owners))
		for i, owner := range r.Owners {
			ownerStrs[i] = owner.String()
		}
		ownersStr = " " + strings.Join(ownerStrs, " ")
	}

	result := fmt.Sprintf("%s%s%s", prefix, r.Pattern, ownersStr)

	if !r.IsValid {
		if r.HasZeroOwners && !r.HasParseError {
			result += " (uses section default owners)"
		} else if r.ParseError != "" {
			result += fmt.Sprintf(" (error: %s)", r.ParseError)
		}
	}

	return result
}

func (s Section) String() string {
	header := fmt.Sprintf("[%s]", s.Name)
	if s.IsOptional {
		header = "^" + header
	}
	if s.RequiredApprovals > 1 {
		header += fmt.Sprintf("[%d]", s.RequiredApprovals)
	}
	if len(s.DefaultOwners) > 0 {
		ownerStrs := make([]string, len(s.DefaultOwners))
		for i, owner := range s.DefaultOwners {
			ownerStrs[i] = owner.String()
		}
		header += " " + strings.Join(ownerStrs, " ")
	}

	if s.IsCombined {
		header += " (combined)"
	}
	if s.ParseError != "" {
		header += fmt.Sprintf(" (error: %s)", s.ParseError)
	}

	return header
}
