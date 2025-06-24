package codeowners

import (
	"fmt"
	"strconv"
	"strings"
)

// parseRule parses a single rule line with enhanced error handling
func (p *Parser) parseRule(line string, lineNumber int) (*Rule, error) {
	rule := &Rule{
		LineNumber: lineNumber,
		IsValid:    true,
	}

	// Handle escaped spaces in paths
	escapedLine := p.handleEscapedSpaces(line)

	// Split the line into pattern and owners
	parts := strings.Fields(escapedLine)
	if len(parts) < 1 {
		return nil, fmt.Errorf("empty rule")
	}

	// Check if this is an exclusion pattern
	if strings.HasPrefix(parts[0], "!") {
		rule.IsExclusion = true
		rule.Pattern = parts[0][1:] // Remove the ! prefix
	} else {
		rule.Pattern = parts[0]
	}

	// Unescape the pattern
	rule.Pattern = p.unescapeSpaces(rule.Pattern)

	// Parse owners (if any)
	if len(parts) > 1 {
		ownersPart := strings.Join(parts[1:], " ")
		ownersPart = p.unescapeSpaces(ownersPart)
		owners, err := p.parseOwners(ownersPart)

		// Always store the owners, even if there was an error
		rule.Owners = owners

		if err != nil {
			rule.ParseError = fmt.Sprintf("error parsing owners: %v", err)
			rule.IsValid = false
			rule.HasParseError = true
		}
	}

	// Check for zero owners
	if len(rule.Owners) == 0 {
		rule.HasZeroOwners = true
	}
	return rule, nil
}

// parseSection parses a section header line with enhanced error handling
func (p *Parser) parseSection(line string, lineNumber int) (*Section, error) {
	section := &Section{
		LineNumber:        lineNumber,
		RequiredApprovals: 1, // Default
	}

	// Check if section is optional (starts with ^[)
	if strings.HasPrefix(line, "^[") {
		section.IsOptional = true
		line = line[1:] // Remove the ^ character
	}

	// Find the closing bracket for section name
	closeBracketIndex := strings.Index(line, "]")
	if closeBracketIndex == -1 {
		return nil, fmt.Errorf("invalid section header: missing closing bracket")
	}

	// Extract section name
	section.Name = strings.TrimSpace(line[1:closeBracketIndex])
	if section.Name == "" {
		return nil, fmt.Errorf("section name cannot be empty")
	}

	// Parse anything after the section name
	remaining := strings.TrimSpace(line[closeBracketIndex+1:])

	// Check for approval count [Section][5]
	if strings.HasPrefix(remaining, "[") && strings.Contains(remaining, "]") {
		endBracket := strings.Index(remaining, "]")
		countStr := remaining[1:endBracket]
		if count, err := strconv.Atoi(countStr); err == nil {
			if count < 1 {
				// GitLab treats 0 or negative as 1
				section.RequiredApprovals = 1
			} else {
				section.RequiredApprovals = count
			}
			remaining = strings.TrimSpace(remaining[endBracket+1:])
		} else {
			// Invalid number, treat as 1 (GitLab behavior)
			section.RequiredApprovals = 1
			// Don't consume the brackets in this case
		}
	}

	// Parse default owners
	if remaining != "" {
		owners, err := p.parseOwners(remaining)
		if err != nil {
			section.ParseError = fmt.Sprintf("error parsing section default owners: %v", err)
		} else {
			section.DefaultOwners = owners
		}
	}

	return section, nil
}

// handleEscapedSpaces handles escaped spaces in the line
func (p *Parser) handleEscapedSpaces(line string) string {
	// Replace escaped spaces with a placeholder
	return strings.ReplaceAll(line, "\\ ", "§ESCAPED_SPACE§")
}

// unescapeSpaces converts escaped space placeholders back to spaces
func (p *Parser) unescapeSpaces(s string) string {
	return strings.Replace(s, "§ESCAPED_SPACE§", " ", -1)
}
