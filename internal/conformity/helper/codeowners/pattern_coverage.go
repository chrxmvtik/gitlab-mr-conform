package codeowners

import "fmt"

// GetPatternCoverage returns coverage information for patterns
func GetPatternCoverage(c *CODEOWNERSFile, paths []string) map[string]PatternCoverage {
	coverage := make(map[string]PatternCoverage)

	// Get all patterns from the CODEOWNERS file
	allPatterns := make(map[string]PatternInfo)

	// Extract patterns from default rules
	for i, rule := range c.DefaultRules {
		key := fmt.Sprintf("Default:%d:%s", i, rule.Pattern)
		allPatterns[key] = PatternInfo{
			Pattern:     rule.Pattern,
			SectionName: "Default",
			LineNumber:  rule.LineNumber,
			IsExclusion: rule.IsExclusion,
		}
	}

	// Extract patterns from sections
	for _, section := range c.Sections {
		for i, rule := range section.Rules {
			key := fmt.Sprintf("%s:%d:%s", section.Name, i, rule.Pattern)
			allPatterns[key] = PatternInfo{
				Pattern:     rule.Pattern,
				SectionName: section.Name,
				LineNumber:  rule.LineNumber,
				IsExclusion: rule.IsExclusion,
			}
		}
	}

	// Check coverage for each pattern
	for key, patternInfo := range allPatterns {
		matchedFiles := []string{}

		for _, file := range paths {

			if NewPatternMatcher().MatchesPattern(patternInfo.Pattern, file) {
				matchedFiles = append(matchedFiles, file)
			}
		}

		coverage[key] = PatternCoverage{
			Pattern:       patternInfo.Pattern,
			SectionName:   patternInfo.SectionName,
			LineNumber:    patternInfo.LineNumber,
			IsExclusion:   patternInfo.IsExclusion,
			MatchedFiles:  matchedFiles,
			CoverageCount: len(matchedFiles),
			CoverageRate:  float64(len(matchedFiles)) / float64(len(paths)) * 100,
		}
	}

	return coverage
}
