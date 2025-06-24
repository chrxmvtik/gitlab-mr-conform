package codeowners

import (
	"bufio"
	"fmt"
	"gitlab-mr-conformity-bot/pkg/logger"
	"io"
	"regexp"
	"strings"
)

// Parser handles the parsing of CODEOWNERS files
type Parser struct {
	emailRegex            *regexp.Regexp
	roleRegex             *regexp.Regexp
	userOrGroupRegex      *regexp.Regexp
	sectionRegex          *regexp.Regexp
	strictValidation      bool
	accessibleOwners      *AccessibleOwners // Map of accessible owners for validation
	caseSensitiveSections bool
	accessLevelMap        map[string]int
	logger                *logger.Logger
}

// NewCodeownersParser creates a new CODEOWNERS parser
func NewCodeownersParser(logger *logger.Logger) *Parser {
	// Default access level mapping
	accessLevelMap := map[string]int{
		"owner":       AccessLevelOwner,
		"owners":      AccessLevelOwner,
		"maintainer":  AccessLevelMaintainer,
		"maintainers": AccessLevelMaintainer,
		"developer":   AccessLevelDeveloper,
		"developers":  AccessLevelDeveloper,
	}
	return &Parser{
		emailRegex:            regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`),
		roleRegex:             regexp.MustCompile(`^@@([a-zA-Z0-9_.][a-zA-Z0-9_.-]*)$`),
		userOrGroupRegex:      regexp.MustCompile(`^@([a-zA-Z0-9_./-][a-zA-Z0-9_.-/-]*)$`),
		sectionRegex:          regexp.MustCompile(`^\^?\[(.*?)\](?:\[(\d+)\])?(.*)$`),
		strictValidation:      true,
		accessibleOwners:      NewAccessibleOwners(),
		caseSensitiveSections: false,
		accessLevelMap:        accessLevelMap,
		logger:                logger,
	}
}

// Parse parses a CODEOWNERS file from a reader
func (p *Parser) Parse(reader io.Reader) (*CODEOWNERSFile, error) {
	codeowners := &CODEOWNERSFile{
		Sections:     []Section{},
		DefaultRules: []Rule{},
		ParseErrors:  []string{},
	}

	scanner := bufio.NewScanner(reader)
	lineNumber := 0
	currentSection := (*Section)(nil)
	sectionMap := make(map[string]*Section) // Track sections by name for combining

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check if this is a section header
		if p.sectionRegex.MatchString(line) {
			section, err := p.parseSection(line, lineNumber)
			if err != nil {
				// Handle unparsable sections according to GitLab behavior
				if currentSection != nil {
					// Add as entry to current section
					rule, ruleErr := p.parseRule(line, lineNumber)
					if ruleErr != nil {
						rule = &Rule{
							Pattern:       line,
							LineNumber:    lineNumber,
							IsValid:       false,
							ParseError:    fmt.Sprintf("unparsable section treated as rule: %v", err),
							HasParseError: true,
						}
					}
					currentSection.Rules = append(currentSection.Rules, *rule)
				} else {
					// Add to default rules
					rule := &Rule{
						Pattern:       line,
						LineNumber:    lineNumber,
						IsValid:       false,
						ParseError:    fmt.Sprintf("unparsable section treated as rule: %v", err),
						HasParseError: true,
					}
					codeowners.DefaultRules = append(codeowners.DefaultRules, *rule)
				}
				continue
			}

			// Handle section combining (case-insensitive by default)
			sectionKey := section.Name
			if !p.caseSensitiveSections {
				sectionKey = strings.ToLower(section.Name)
			}

			if existingSection, exists := sectionMap[sectionKey]; exists {
				// Combine with existing section
				existingSection.Rules = append(existingSection.Rules, section.Rules...)
				existingSection.IsCombined = true
				currentSection = existingSection
			} else {
				// New section
				codeowners.Sections = append(codeowners.Sections, *section)
				currentSection = &codeowners.Sections[len(codeowners.Sections)-1]
				sectionMap[sectionKey] = currentSection
			}
			continue
		}

		// Parse rule
		rule, err := p.parseRule(line, lineNumber)
		if err != nil {
			fmt.Printf("line %d: %v", lineNumber, err)
			codeowners.ParseErrors = append(codeowners.ParseErrors,
				fmt.Sprintf("line %d: %v", lineNumber, err))
			// Still add the rule but mark it as invalid
			rule = &Rule{
				Pattern:       line,
				LineNumber:    lineNumber,
				IsValid:       false,
				ParseError:    err.Error(),
				HasParseError: true,
			}
		}

		// Add rule to current section or default rules
		if currentSection != nil {
			currentSection.Rules = append(currentSection.Rules, *rule)
			p.logger.Debug("Added rule to section", "rule", rule.Pattern, "section", currentSection.Name, "sectionRulesCount", len(currentSection.Rules))
		} else {
			codeowners.DefaultRules = append(codeowners.DefaultRules, *rule)
			p.logger.Debug("Added rule to default rules", "rule", rule.Pattern, "section", "Default", "sectionRulesCount", len(codeowners.DefaultRules))
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	// Post-process to validate owners and handle zero-owner rules
	p.postProcessValidation(codeowners)

	return codeowners, nil
}

// SetStrictValidation enables/disables strict validation
func (p *Parser) SetStrictValidation(strict bool) {
	p.strictValidation = strict
}

// SetCaseSensitiveSections enables/disables case-sensitive section names
func (p *Parser) SetCaseSensitiveSections(caseSensitive bool) {
	p.caseSensitiveSections = caseSensitive
}
