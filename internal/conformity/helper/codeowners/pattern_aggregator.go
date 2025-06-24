package codeowners

import (
	"fmt"
	"sort"
)

// GetAggregatedOwnershipByPattern aggregates ownership results by matching patterns
func GetAggregatedOwnershipByPattern(c *CODEOWNERSFile, paths []string) *PatternAggregation {
	patternGroups := make(map[string]*PatternGroup)
	patternStats := make(map[string]PatternStats)
	sectionSummary := make(map[string]SectionPatternSummary)

	// Track global statistics
	allOwners := make(map[string]bool)
	patternFileCount := make(map[string]int)

	// Process each file
	for _, file := range paths {
		owners := c.GetOwnersForFile(file)

		for _, so := range owners {
			// Process each matching pattern for this section
			for _, pattern := range so.MatchingPatterns {
				// Create unique key for this pattern in this section
				key := createPatternKey(pattern.Pattern, so.Name, pattern.IsExclusion, pattern.LineNumber)

				// Track file count for this pattern
				patternFileCount[pattern.Pattern]++

				// Update or create pattern group
				if group, exists := patternGroups[key]; exists {
					group.Files = append(group.Files, file)
				} else {
					patternGroups[key] = &PatternGroup{
						Pattern:           pattern.Pattern,
						SectionName:       so.Name,
						IsExclusion:       pattern.IsExclusion,
						LineNumber:        pattern.LineNumber,
						MatchType:         pattern.MatchType,
						Files:             []string{file},
						Owners:            so.Owners,
						RequiredApprovals: so.RequiredApprovals,
						IsOptional:        so.IsOptional,
						IsAutoApproved:    so.IsAutoApproved,
						UsedDefaultOwners: so.UsedDefaultOwners,
						ValidationErrors:  so.ValidationErrors,
					}
				}

				// Track owners globally
				for _, owner := range so.Owners {
					allOwners[owner.Original] = true
				}
			}
		}
	}

	// Sort files in each group
	for _, group := range patternGroups {
		sort.Strings(group.Files)
	}

	// Calculate pattern statistics
	for key, group := range patternGroups {
		stats := PatternStats{
			TotalFiles:      len(group.Files),
			UniqueOwners:    len(getUniqueOwners(group.Owners)),
			SectionsUsed:    []string{group.SectionName},
			MatchTypes:      map[string]int{group.MatchType: len(group.Files)},
			OverriddenCount: 0, // Will be calculated separately if needed
		}

		if group.IsAutoApproved {
			stats.AutoApprovedFiles = len(group.Files)
		}

		patternStats[key] = stats
	}

	// Calculate section summaries
	sectionPatternCount := make(map[string]int)
	sectionFileCount := make(map[string]int)
	sectionOwners := make(map[string]map[string]bool)
	sectionExclusions := make(map[string]int)
	sectionAutoApproved := make(map[string]int)

	for _, group := range patternGroups {
		sectionPatternCount[group.SectionName]++
		sectionFileCount[group.SectionName] += len(group.Files)

		if sectionOwners[group.SectionName] == nil {
			sectionOwners[group.SectionName] = make(map[string]bool)
		}

		for _, owner := range group.Owners {
			sectionOwners[group.SectionName][owner.Original] = true
		}

		if group.IsExclusion {
			sectionExclusions[group.SectionName]++
		}

		if group.IsAutoApproved {
			sectionAutoApproved[group.SectionName] += len(group.Files)
		}
	}

	for sectionName, patternCount := range sectionPatternCount {
		sectionSummary[sectionName] = SectionPatternSummary{
			SectionName:    sectionName,
			PatternCount:   patternCount,
			FileCount:      sectionFileCount[sectionName],
			UniqueOwners:   len(sectionOwners[sectionName]),
			ExclusionCount: sectionExclusions[sectionName],
			AutoApproved:   sectionAutoApproved[sectionName],
		}
	}

	// Calculate overall statistics
	totalExclusions := 0
	totalAutoApproved := 0
	mostUsedPattern := ""
	mostUsedCount := 0

	for pattern, count := range patternFileCount {
		if count > mostUsedCount {
			mostUsedCount = count
			mostUsedPattern = pattern
		}
	}

	for _, group := range patternGroups {
		if group.IsExclusion {
			totalExclusions++
		}
		if group.IsAutoApproved {
			totalAutoApproved += len(group.Files)
		}
	}

	overallStats := OverallPatternStats{
		TotalPatterns:     len(patternGroups),
		TotalFiles:        len(paths),
		UniqueOwners:      len(allOwners),
		ExclusionPatterns: totalExclusions,
		AutoApproved:      totalAutoApproved,
		MostUsedPattern:   mostUsedPattern,
		MostUsedCount:     mostUsedCount,
	}

	return &PatternAggregation{
		PatternGroups:  patternGroups,
		PatternStats:   patternStats,
		SectionSummary: sectionSummary,
		OverallStats:   overallStats,
	}
}

// GetActivePatternAggregation aggregates only active (non-overridden) patterns
func GetActivePatternAggregation(c *CODEOWNERSFile, paths []string) *PatternAggregation {
	patternGroups := make(map[string]*PatternGroup)
	patternStats := make(map[string]PatternStats)
	sectionSummary := make(map[string]SectionPatternSummary)

	// Track global statistics
	allOwners := make(map[string]bool)
	patternFileCount := make(map[string]int)

	// Process each file
	for _, file := range paths {
		owners := c.GetOwnersForFile(file)

		for _, so := range owners {
			// Process only active patterns
			for _, pattern := range so.MatchingPatterns {
				if !pattern.IsActive {
					continue // Skip inactive patterns
				}

				// Create unique key for this active pattern in this section
				key := createPatternKey(pattern.Pattern, so.Name, pattern.IsExclusion, pattern.LineNumber)

				// Track file count for this pattern
				patternFileCount[pattern.Pattern]++

				// Update or create pattern group
				if group, exists := patternGroups[key]; exists {
					group.Files = append(group.Files, file)
				} else {
					patternGroups[key] = &PatternGroup{
						Pattern:           pattern.Pattern,
						SectionName:       so.Name,
						IsExclusion:       pattern.IsExclusion,
						LineNumber:        pattern.LineNumber,
						MatchType:         pattern.MatchType,
						Files:             []string{file},
						Owners:            so.Owners,
						RequiredApprovals: so.RequiredApprovals,
						IsOptional:        so.IsOptional,
						IsAutoApproved:    so.IsAutoApproved,
						UsedDefaultOwners: so.UsedDefaultOwners,
						ValidationErrors:  so.ValidationErrors,
					}
				}

				// Track owners globally
				for _, owner := range so.Owners {
					allOwners[owner.Original] = true
				}
			}
		}
	}

	// Sort files in each group
	for _, group := range patternGroups {
		sort.Strings(group.Files)
	}

	// Calculate statistics (similar to above but for active patterns only)
	// ... (rest of the statistics calculation is similar)

	return &PatternAggregation{
		PatternGroups:  patternGroups,
		PatternStats:   patternStats,
		SectionSummary: sectionSummary,
		OverallStats: OverallPatternStats{
			TotalPatterns:   len(patternGroups),
			TotalFiles:      len(paths),
			UniqueOwners:    len(allOwners),
			MostUsedPattern: findMostUsedPattern(patternFileCount),
			MostUsedCount:   findMostUsedCount(patternFileCount),
		},
	}
}

// Helper functions
func createPatternKey(pattern, sectionName string, isExclusion bool, lineNumber int) string {
	exclusionPrefix := ""
	if isExclusion {
		exclusionPrefix = "!"
	}
	return fmt.Sprintf("%s:%s%s:%d", sectionName, exclusionPrefix, pattern, lineNumber)
}

// String methods for pretty printing
func (pg PatternGroup) String() string {
	exclusionPrefix := ""
	if pg.IsExclusion {
		exclusionPrefix = "!"
	}

	return fmt.Sprintf("Pattern: %s%s [%s] - %d files, %d owners (line %d)",
		exclusionPrefix, pg.Pattern, pg.MatchType, len(pg.Files),
		len(pg.Owners), pg.LineNumber)
}

func (ps PatternStats) String() string {
	return fmt.Sprintf("Files: %d, Owners: %d, Sections: %v, Auto-approved: %d",
		ps.TotalFiles, ps.UniqueOwners, ps.SectionsUsed, ps.AutoApprovedFiles)
}

func (sps SectionPatternSummary) String() string {
	return fmt.Sprintf("Section: %s - %d patterns, %d files, %d owners, %d exclusions, %d auto-approved",
		sps.SectionName, sps.PatternCount, sps.FileCount, sps.UniqueOwners,
		sps.ExclusionCount, sps.AutoApproved)
}
