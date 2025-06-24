package codeowners

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar"
	"github.com/gobwas/glob"
)

// PatternMatcher handles different types of pattern matching
type PatternMatcher struct {
	compiledGlobs map[string]glob.Glob
	useDoublestar bool // Whether to use doublestar for ** patterns
}

// NewPatternMatcher creates a new pattern matcher
func NewPatternMatcher() *PatternMatcher {
	return &PatternMatcher{
		compiledGlobs: make(map[string]glob.Glob),
		useDoublestar: true,
	}
}

// CompilePattern pre-compiles a pattern for better performance
func (pm *PatternMatcher) CompilePattern(pattern string) error {
	if _, exists := pm.compiledGlobs[pattern]; exists {
		return nil // Already compiled
	}

	// Use different strategies based on pattern complexity
	if strings.Contains(pattern, "**") && pm.useDoublestar {
		// Don't pre-compile doublestar patterns as they're handled differently
		return nil
	}

	compiled, err := glob.Compile(pattern)
	if err != nil {
		return fmt.Errorf("failed to compile pattern %s: %w", pattern, err)
	}

	pm.compiledGlobs[pattern] = compiled
	return nil
}

// MatchesPattern checks if a file path matches a pattern using the best available method
func (pm *PatternMatcher) MatchesPattern(pattern, filePath string) bool {
	// Normalize paths
	filePath = filepath.Clean(filepath.ToSlash(filePath))

	// Handle absolute patterns
	if strings.HasPrefix(pattern, "/") {
		return pm.matchAbsolutePattern(pattern[1:], filePath)
	}

	// Handle directory patterns
	if strings.HasSuffix(pattern, "/") {
		return pm.matchDirectoryPattern(pattern, filePath)
	}

	// Handle relative patterns - try matching at any level
	return pm.matchRelativePattern(pattern, filePath)
}

// GetMatchType determines the type of pattern match
func (pm *PatternMatcher) GetMatchType(pattern, filePath string) string {
	if strings.Contains(pattern, "**") {
		return "globstar"
	}

	if strings.HasSuffix(pattern, "/") {
		return "directory"
	}

	// Check if it's an exact match (no wildcards)
	if !strings.ContainsAny(pattern, "*?[{") {
		return "exact"
	}

	return "glob"
}

// matchAbsolutePattern matches patterns that start with /
func (pm *PatternMatcher) matchAbsolutePattern(pattern, filePath string) bool {
	if strings.Contains(pattern, "**") && pm.useDoublestar {
		matched, _ := doublestar.Match(pattern, filePath)
		return matched
	}

	if compiled, exists := pm.compiledGlobs[pattern]; exists {
		return compiled.Match(filePath)
	}

	// Fallback to filepath.Match
	matched, _ := filepath.Match(pattern, filePath)
	return matched
}

// matchDirectoryPattern matches directory patterns (ending with /)
func (pm *PatternMatcher) matchDirectoryPattern(pattern, filePath string) bool {
	dirPattern := strings.TrimSuffix(pattern, "/")

	// Check if file is within this directory
	return strings.HasPrefix(filePath, dirPattern+"/") ||
		filePath == dirPattern
}

// matchRelativePattern matches relative patterns at any directory level
func (pm *PatternMatcher) matchRelativePattern(pattern, filePath string) bool {
	// Try exact match first
	if pm.matchExact(pattern, filePath) {
		return true
	}

	// Try matching against all possible subpaths
	pathParts := strings.Split(filePath, "/")
	for i := 0; i < len(pathParts); i++ {
		subPath := strings.Join(pathParts[i:], "/")
		if pm.matchExact(pattern, subPath) {
			return true
		}
	}

	return false
}

// matchExact performs exact pattern matching
func (pm *PatternMatcher) matchExact(pattern, path string) bool {
	if strings.Contains(pattern, "**") && pm.useDoublestar {
		matched, _ := doublestar.Match(pattern, path)
		return matched
	}

	if compiled, exists := pm.compiledGlobs[pattern]; exists {
		return compiled.Match(path)
	}

	// Fallback to filepath.Match
	matched, _ := filepath.Match(pattern, path)
	return matched
}
