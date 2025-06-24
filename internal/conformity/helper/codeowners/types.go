package codeowners

import (
	"gitlab-mr-conformity-bot/internal/conformity/helper/common"

	gitlabapi "gitlab.com/gitlab-org/api/client-go"
)

// Owner represents a code owner (user, group, or role)
type Owner struct {
	Type     OwnerType
	Name     string
	IsEmail  bool
	IsRole   bool
	IsGroup  bool
	IsNested bool
	IsValid  bool   // Track if owner is accessible/valid
	Original string // Original string representation
}

// OwnerType represents the type of owner
type OwnerType int

const (
	OwnerTypeUser OwnerType = iota
	OwnerTypeGroup
	OwnerTypeRole
)

// AccessibleOwners represents the separated accessible owners by type
type AccessibleOwners struct {
	Users     map[string]bool // @username format (without @)
	Groups    map[string]bool // @group/subgroup format (without @)
	Roles     map[string]bool // @@role format (without @@)
	Emails    map[string]bool // email@domain.com format
	RoleLevel int
}

// Rule represents a single CODEOWNERS rule
type Rule struct {
	Pattern       string
	IsExclusion   bool
	Owners        []Owner
	LineNumber    int
	IsValid       bool   // Track if rule has valid owners
	HasZeroOwners bool   // Track if rule has zero accessible owners
	ParseError    string // Track any parsing errors
	HasParseError bool
}

// Section represents a section in the CODEOWNERS file
type Section struct {
	Name              string
	IsOptional        bool
	RequiredApprovals int
	DefaultOwners     []Owner
	Rules             []Rule
	LineNumber        int
	ParseError        string // Track section parsing errors
	IsCombined        bool   // Track if this section was combined with others
}

// CODEOWNERSFile represents the parsed CODEOWNERS file
type CODEOWNERSFile struct {
	Sections       []Section
	DefaultRules   []Rule // Rules before any section
	GlobalComment  []string
	ParseErrors    []string // Global parsing errors
	patternMatcher *PatternMatcher
}

// Enhanced SectionOwnership to include matching patterns and rule details
type SectionOwnership struct {
	Name              string
	Owners            []Owner
	RequiredApprovals int
	IsOptional        bool
	IsAutoApproved    bool
	ValidationErrors  []string
	UsedDefaultOwners bool
	MatchingPatterns  []MatchingPattern // New: track which patterns matched
}

// MatchingPattern represents a pattern that matched the file
type MatchingPattern struct {
	Pattern      string
	IsExclusion  bool
	LineNumber   int
	RuleIndex    int              // Index of the rule within the section
	MatchType    string           // "exact", "glob", "directory", "globstar"
	IsActive     bool             // Whether this pattern is the active one (not overridden)
	OverriddenBy *MatchingPattern // If overridden, which pattern overrode it
}

// AccessLevel constants for GitLab-style role access levels
const (
	AccessLevelGuest      = 10
	AccessLevelPlanner    = 15
	AccessLevelReporter   = 20
	AccessLevelDeveloper  = 30
	AccessLevelMaintainer = 40
	AccessLevelOwner      = 50
	AccessLevelAdmin      = 60
)

// NewAccessibleOwners creates a new AccessibleOwners instance
func NewAccessibleOwners() *AccessibleOwners {
	return &AccessibleOwners{
		Users:     make(map[string]bool),
		Groups:    make(map[string]bool),
		Roles:     make(map[string]bool),
		Emails:    make(map[string]bool),
		RoleLevel: 0,
	}
}

// Summary structs
type OwnerApprovalStatus struct {
	Owner        Owner
	HasApproved  bool
	ApprovalInfo *common.ApprovalInfo
}

type PatternApprovalSummary struct {
	Pattern          *PatternGroup
	OwnerStatuses    []OwnerApprovalStatus
	ApprovedCount    int
	RequiredCount    int
	RemainingCount   int
	IsFullyApproved  bool
	IsAutoApproved   bool
	IsOptional       bool
	IsExclusion      bool
	AllowedApprovers []string
}

type CodeOwnersSummary struct {
	Patterns            []PatternApprovalSummary
	TotalApproved       int
	TotalRequired       int
	AllPatternsApproved bool
	Members             []*gitlabapi.ProjectMember
}

// Merged section summary for grouping patterns by section AND owners
type MergedSectionSummary struct {
	SectionName      string
	Patterns         []string
	ApprovedCount    int
	RequiredCount    int
	IsFullyApproved  bool
	IsAutoApproved   bool
	IsOptional       bool
	IsExclusion      bool
	AllowedApprovers []string
	OwnersSignature  string
	PatternSummaries []PatternApprovalSummary
}

// PatternGroup represents files grouped by matching pattern
type PatternGroup struct {
	Pattern           string
	SectionName       string
	IsExclusion       bool
	LineNumber        int
	MatchType         string
	Files             []string
	Owners            []Owner
	RequiredApprovals int
	IsOptional        bool
	IsAutoApproved    bool
	UsedDefaultOwners bool
	ValidationErrors  []string
}

// PatternAggregation represents the complete aggregation by patterns
type PatternAggregation struct {
	PatternGroups  map[string]*PatternGroup
	PatternStats   map[string]PatternStats
	SectionSummary map[string]SectionPatternSummary
	OverallStats   OverallPatternStats
}

// PatternStats provides statistics for a specific pattern
type PatternStats struct {
	TotalFiles        int
	UniqueOwners      int
	SectionsUsed      []string
	MatchTypes        map[string]int
	OverriddenCount   int
	AutoApprovedFiles int
}

// SectionPatternSummary provides pattern summary per section
type SectionPatternSummary struct {
	SectionName    string
	PatternCount   int
	FileCount      int
	UniqueOwners   int
	ExclusionCount int
	AutoApproved   int
}

// OverallPatternStats provides overall statistics
type OverallPatternStats struct {
	TotalPatterns     int
	TotalFiles        int
	UniqueOwners      int
	ExclusionPatterns int
	AutoApproved      int
	MostUsedPattern   string
	MostUsedCount     int
}

// OwnershipGroup represents a group of files with the same ownership requirements
type OwnershipGroup struct {
	SectionName       string
	Owners            []Owner
	RequiredApprovals int
	Files             []string
	MatchingPattern   []MatchingPattern
}

// Helper types for pattern coverage
type PatternInfo struct {
	Pattern     string
	SectionName string
	LineNumber  int
	IsExclusion bool
}

type PatternCoverage struct {
	Pattern       string
	SectionName   string
	LineNumber    int
	IsExclusion   bool
	MatchedFiles  []string
	CoverageCount int
	CoverageRate  float64 // Percentage of files matched
}

type ValidationErrors struct {
	LineNumber int
	Errors     []string
}
