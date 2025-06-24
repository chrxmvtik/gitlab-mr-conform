package codeowners

import (
	"fmt"
	"gitlab-mr-conformity-bot/internal/conformity/helper/common"
	"sort"
	"strings"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

// Main function to create the summary - now accepts members parameter
func CreateCodeOwnersSummary(codeowners []*PatternGroup, approvals *common.Approvals, members []*gitlabapi.ProjectMember) *CodeOwnersSummary {
	summary := &CodeOwnersSummary{
		Patterns: make([]PatternApprovalSummary, 0, len(codeowners)),
		Members:  members,
	}

	for _, pattern := range codeowners {
		// Skip exclusion patterns - they should not be included in approval checks
		if pattern.IsExclusion {
			continue
		}

		patternSummary := createPatternSummary(pattern, approvals, members)
		summary.Patterns = append(summary.Patterns, patternSummary)

		// Only count towards total if the pattern is fully approved
		if patternSummary.IsFullyApproved {
			summary.TotalApproved++
		}

		// Only count non-optional and non-auto-approved patterns towards required total
		if !patternSummary.IsOptional && !patternSummary.IsAutoApproved {
			summary.TotalRequired++
		}
	}

	// Check if all patterns are approved
	summary.AllPatternsApproved = true
	for _, pattern := range summary.Patterns {
		if !pattern.IsFullyApproved {
			summary.AllPatternsApproved = false
			break
		}
	}

	return summary
}

func createPatternSummary(pattern *PatternGroup, approvals *common.Approvals, members []*gitlabapi.ProjectMember) PatternApprovalSummary {
	summary := PatternApprovalSummary{
		Pattern:        pattern,
		RequiredCount:  pattern.RequiredApprovals,
		IsAutoApproved: pattern.IsAutoApproved,
		IsOptional:     pattern.IsOptional,
		IsExclusion:    pattern.IsExclusion,
		OwnerStatuses:  make([]OwnerApprovalStatus, 0, len(pattern.Owners)),
	}

	// Exclusion patterns should never be processed for approvals
	if pattern.IsExclusion {
		// This should not happen since we filter exclusions in CreateCodeOwnersSummary,
		// but added as a safety check
		summary.IsFullyApproved = true
		summary.ApprovedCount = 0
		summary.RemainingCount = 0
		return summary
	}

	// If the pattern is auto-approved or optional, mark it as fully approved immediately
	if pattern.IsAutoApproved || pattern.IsOptional {
		summary.IsFullyApproved = true
		summary.ApprovedCount = summary.RequiredCount // Set to required count to show as satisfied
		summary.RemainingCount = 0

		// Still process owners for display purposes
		for _, owner := range pattern.Owners {
			ownerStatus := OwnerApprovalStatus{
				Owner: owner,
			}

			// Find if this owner has approved (for display)
			if approvals != nil && approvals.ApprovalsInfo != nil {
				for userID, approval := range approvals.ApprovalsInfo {
					if matchesOwner(owner, approval, members) {
						ownerStatus.HasApproved = (approval.Status == "approved")
						ownerStatus.ApprovalInfo = &common.ApprovalInfo{
							UserID:    userID,
							Username:  approval.Username,
							Status:    approval.Status,
							UpdatedAt: approval.UpdatedAt,
						}
						break
					}
				}
			}

			summary.OwnerStatuses = append(summary.OwnerStatuses, ownerStatus)

			// Add to allowed approvers list - expand roles to individual members
			if owner.IsRole {
				roleMembers := getRoleMembers(owner.Name, members)
				summary.AllowedApprovers = append(summary.AllowedApprovers, roleMembers...)
			} else {
				// Since owners are pre-filtered to accessible members, we can directly add them
				summary.AllowedApprovers = append(summary.AllowedApprovers, owner.Name)
			}
		}

		return summary
	}

	// Build allowed approvers list - since owners are pre-filtered, all should be valid
	for _, owner := range pattern.Owners {
		if owner.IsRole {
			roleMembers := getRoleMembers(owner.Name, members)
			summary.AllowedApprovers = append(summary.AllowedApprovers, roleMembers...)
		} else {
			summary.AllowedApprovers = append(summary.AllowedApprovers, owner.Name)
		}
	}

	// Check each owner's approval status for regular patterns
	for _, owner := range pattern.Owners {
		ownerStatus := OwnerApprovalStatus{
			Owner: owner,
		}

		// Find if this owner has approved
		if approvals != nil && approvals.ApprovalsInfo != nil {
			for userID, approval := range approvals.ApprovalsInfo {
				if matchesOwner(owner, approval, members) {
					ownerStatus.HasApproved = (approval.Status == "approved")
					ownerStatus.ApprovalInfo = &common.ApprovalInfo{
						UserID:    userID,
						Username:  approval.Username,
						Status:    approval.Status,
						UpdatedAt: approval.UpdatedAt,
					}

					// Count valid approvals - since owners are pre-filtered, all approvals are valid
					if approval.Status == "approved" {
						summary.ApprovedCount++
					}
					break
				}
			}
		}

		summary.OwnerStatuses = append(summary.OwnerStatuses, ownerStatus)
	}

	summary.RemainingCount = max(0, summary.RequiredCount-summary.ApprovedCount)
	summary.IsFullyApproved = summary.ApprovedCount >= summary.RequiredCount

	return summary
}

// common function to get members that belong to a specific role based on access level
func getRoleMembers(roleName string, members []*gitlabapi.ProjectMember) []string {
	if members == nil {
		return []string{}
	}

	// Map role names to required access levels
	requiredLevel := getRequiredAccessLevel(roleName)
	if requiredLevel == -1 {
		return []string{} // Unknown role
	}

	var roleMembers []string
	for _, member := range members {
		// Check if member has the required access level
		if int(member.AccessLevel) == requiredLevel {
			roleMembers = append(roleMembers, member.Username)
		}
	}

	return roleMembers
}

// common function to map role names to access levels
func getRequiredAccessLevel(roleName string) int {
	// Remove @@ prefix if present and convert to lowercase
	cleanName := strings.ToLower(strings.TrimPrefix(roleName, "@@"))
	cleanName = strings.TrimPrefix(cleanName, "@") // Also handle single @

	// Map common role patterns to GitLab access levels
	accessLevelMap := map[string]int{
		"owner":       50, // Owner
		"owners":      50,
		"maintainer":  40, // Maintainer
		"maintainers": 40,
		"developer":   30, // Developer
		"developers":  30,
	}

	// Check for exact matches first
	if level, exists := accessLevelMap[cleanName]; exists {
		return level
	}

	return -1 // Unknown role
}

// Simplified common function to match owner with approval - uses Owner struct properties
func matchesOwner(owner Owner, approval common.ApprovalInfo, members []*gitlabapi.ProjectMember) bool {
	// Handle email matching
	if owner.IsEmail {
		return strings.EqualFold(owner.Name, approval.Username) ||
			strings.Contains(strings.ToLower(approval.Username), strings.ToLower(owner.Name))
	}

	// Handle role matching - use the IsRole property from Owner struct
	if owner.IsRole {
		roleMembers := getRoleMembers(owner.Name, members)
		for _, member := range roleMembers {
			if strings.EqualFold(member, approval.Username) {
				return true
			}
		}
		return false
	}

	// For individual usernames
	return strings.EqualFold(owner.Name, approval.Username)
}

// Generate aggregated markdown table with merged sections (by section name AND owners)
func (s *CodeOwnersSummary) GenerateAggregatedOutput() (string, string) {
	var aggregatedTable strings.Builder
	var needsApprovals bool

	// Group patterns by section name AND owners signature
	sectionMap := make(map[string]*MergedSectionSummary)
	sectionOrder := []string{}

	var validationErrors []ValidationErrors

	for _, pattern := range s.Patterns {

		// Skip exclusion patterns in aggregated output
		if pattern.IsExclusion {
			continue
		}

		if len(pattern.Pattern.ValidationErrors) > 0 {
			validationErrors = append(validationErrors, ValidationErrors{
				LineNumber: pattern.Pattern.LineNumber,
				Errors:     pattern.Pattern.ValidationErrors,
			})
		}

		// Create a unique key combining section name and owners
		ownersNames := make([]string, len(pattern.AllowedApprovers))
		copy(ownersNames, pattern.AllowedApprovers)
		sort.Strings(ownersNames)
		ownersSignature := strings.Join(ownersNames, "|")

		// Create unique key for grouping
		groupKey := fmt.Sprintf("%s::%s", pattern.Pattern.SectionName, ownersSignature)

		if _, exists := sectionMap[groupKey]; !exists {
			sectionMap[groupKey] = &MergedSectionSummary{
				SectionName:      pattern.Pattern.SectionName,
				Patterns:         []string{},
				RequiredCount:    pattern.Pattern.RequiredApprovals,
				AllowedApprovers: make([]string, len(pattern.AllowedApprovers)),
				IsAutoApproved:   pattern.Pattern.IsAutoApproved,
				IsOptional:       pattern.Pattern.IsOptional,
				IsExclusion:      pattern.Pattern.IsExclusion,
				OwnersSignature:  ownersSignature,
				PatternSummaries: []PatternApprovalSummary{},
			}
			copy(sectionMap[groupKey].AllowedApprovers, pattern.AllowedApprovers)
			sectionOrder = append(sectionOrder, groupKey)
		}

		section := sectionMap[groupKey]
		section.Patterns = append(section.Patterns, pattern.Pattern.Pattern)
		section.PatternSummaries = append(section.PatternSummaries, pattern)

		// Count approvals from this section's owners only
		section.ApprovedCount = countApprovalsForOwners(section.AllowedApprovers, s.getApprovals())

		// Update approval status - auto-approved and optional sections are always considered approved
		// Also check if any pattern in this section is auto-approved
		isAutoApproved := section.IsAutoApproved || section.IsOptional || len(section.AllowedApprovers) == 0
		for _, patternSummary := range section.PatternSummaries {
			if patternSummary.IsAutoApproved {
				isAutoApproved = true
				break
			}
		}
		section.IsAutoApproved = isAutoApproved
		section.IsFullyApproved = section.ApprovedCount >= section.RequiredCount || isAutoApproved || len(section.AllowedApprovers) == 0
	}

	// Build the complete table with merged sections
	aggregatedTable.WriteString("\n\n| | Code owners | Approvals | Allowed approvers |\n")
	aggregatedTable.WriteString("| --- | --- | --- | --- |\n")

	for _, groupKey := range sectionOrder {
		section := sectionMap[groupKey]

		// Set checkbox based on approval status
		checkbox := "[ ]"
		if section.IsFullyApproved {
			checkbox = "[x]"
		}

		// Format patterns
		var patternsDisplay string
		if len(section.Patterns) == 1 {
			patternsDisplay = fmt.Sprintf("``%s``", section.Patterns[0])
		} else {
			patternsList := make([]string, len(section.Patterns))
			for i, pattern := range section.Patterns {
				patternsList[i] = fmt.Sprintf("``%s``", pattern)
			}
			patternsDisplay = strings.Join(patternsList, "<br>")
		}

		for i, val := range section.AllowedApprovers {
			section.AllowedApprovers[i] = "@" + val
		}

		var approvals string
		if section.IsOptional {
			approvals = "Optional"
		} else if section.IsAutoApproved {
			approvals = "Auto-approved"
		} else {
			approvals = fmt.Sprintf("%d of %d", section.ApprovedCount, section.RequiredCount)
		}
		// Add section row
		aggregatedTable.WriteString(fmt.Sprintf(
			"|<ul><li>%s </li></ul>| <sub>%s</sub><br>%s | %s | %s |\n",
			checkbox,
			section.SectionName,
			patternsDisplay,
			approvals,
			strings.Join(section.AllowedApprovers, ", "),
		))

		if !section.IsFullyApproved {
			needsApprovals = true
		}
	}

	// Return aggregated table and suggestion
	suggestion := ""
	if needsApprovals {
		suggestion = "Wait for required approvals before merging\n"
	}

	totalErrors := 0
	for _, ve := range validationErrors {
		totalErrors += len(ve.Errors)
	}

	if totalErrors > 0 {
		suggestion += "\n> **🚨 Syntax errors:**\n"

		for _, ve := range validationErrors {
			// Process each individual error string
			for _, err := range ve.Errors {
				if strings.TrimSpace(err) != "" { // Skip empty strings
					suggestion += fmt.Sprintf("> - Line %d: %+v\n", ve.LineNumber, strings.TrimSpace(err))
				}
			}
		}
	}
	return aggregatedTable.String(), suggestion
}

// common function to count approvals for specific owners
func countApprovalsForOwners(allowedApprovers []string, approvals *common.Approvals) int {
	if approvals == nil || approvals.ApprovalsInfo == nil {
		return 0
	}

	count := 0
	for _, approval := range approvals.ApprovalsInfo {
		if approval.Status == "approved" {
			for _, approver := range allowedApprovers {
				if matchesApprover(approver, approval) {
					count++
					break
				}
			}
		}
	}
	return count
}

// common function to match approver with approval
func matchesApprover(approver string, approval common.ApprovalInfo) bool {
	return strings.EqualFold(approver, approval.Username)
}

// common function to get approvals from summary
func (s *CodeOwnersSummary) getApprovals() *common.Approvals {
	approvals := &common.Approvals{
		ApprovalsInfo: make(map[int]common.ApprovalInfo),
	}

	// Collect all approvals from pattern summaries
	for _, pattern := range s.Patterns {
		for _, ownerStatus := range pattern.OwnerStatuses {
			if ownerStatus.ApprovalInfo != nil {
				approvals.ApprovalsInfo[ownerStatus.ApprovalInfo.UserID] = *ownerStatus.ApprovalInfo
			}
		}
	}

	return approvals
}

// Generate markdown table for all patterns (keeping for backward compatibility)
func (s *CodeOwnersSummary) GenerateMarkdownTable() []string {
	aggregatedError, suggestion := s.GenerateAggregatedOutput()

	results := []string{aggregatedError}
	if suggestion != "" {
		results = append(results, suggestion)
	}

	return results
}

// Generate detailed report
func (s *CodeOwnersSummary) GenerateDetailedReport() string {
	var report strings.Builder

	report.WriteString("# Code Owners Approval Summary\n\n")
	report.WriteString(fmt.Sprintf("**Overall Status:** %d of %d required approvals received\n\n",
		s.TotalApproved, s.TotalRequired))

	for i, pattern := range s.Patterns {
		// Skip exclusion patterns in detailed report
		if pattern.IsExclusion {
			continue
		}

		report.WriteString(fmt.Sprintf("## Pattern %d: %s\n", i+1, pattern.Pattern.SectionName))
		report.WriteString(fmt.Sprintf("**Pattern:** `%s`\n", pattern.Pattern.Pattern))

		if pattern.IsAutoApproved {
			report.WriteString("**Status:** Auto-approved\n")
			report.WriteString("✅ **Auto-Approved**\n\n")
		} else if pattern.IsOptional {
			report.WriteString("**Status:** Optional\n")
			report.WriteString("✅ **Optional**\n\n")
		} else {
			report.WriteString(fmt.Sprintf("**Status:** %d of %d required approvals\n",
				pattern.ApprovedCount, pattern.RequiredCount))

			if pattern.IsFullyApproved {
				report.WriteString("✅ **Fully Approved**\n\n")
			} else {
				report.WriteString(fmt.Sprintf("❌ **Needs %d more approval(s)**\n\n", pattern.RemainingCount))
			}
		}

		report.WriteString("### Owner Status:\n")
		for _, ownerStatus := range pattern.OwnerStatuses {
			status := "❌ Not Approved"
			details := ""

			if ownerStatus.HasApproved {
				status = "✅ Approved"
				if ownerStatus.ApprovalInfo != nil {
					details = fmt.Sprintf(" (by %s)", ownerStatus.ApprovalInfo.Username)
				}
			}

			ownerDisplay := ownerStatus.Owner.Name
			if ownerStatus.Owner.IsRole {
				roleMembers := getRoleMembers(ownerStatus.Owner.Name, s.Members)
				if len(roleMembers) > 0 {
					ownerDisplay = fmt.Sprintf("%s (%s)", ownerStatus.Owner.Name, strings.Join(roleMembers, ", "))
				}
			}

			report.WriteString(fmt.Sprintf("- **%s**: %s%s\n",
				ownerDisplay, status, details))
		}
		report.WriteString("\n")
	}

	return report.String()
}

// Utility function for max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
