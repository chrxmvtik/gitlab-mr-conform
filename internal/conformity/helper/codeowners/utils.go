package codeowners

import (
	"path/filepath"
	"strings"
)

// PathUtils provides additional path manipulation utilities
type PathUtils struct{}

// NormalizePath normalizes a file path for consistent matching
func (pu *PathUtils) NormalizePath(path string) string {
	// Convert to forward slashes
	path = filepath.ToSlash(path)
	// Clean the path
	path = filepath.Clean(path)
	// Remove leading slash for relative matching
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	return path
}

// IsWithinDirectory checks if a file is within a specific directory
func (pu *PathUtils) IsWithinDirectory(filePath, dirPath string) bool {
	filePath = pu.NormalizePath(filePath)
	dirPath = pu.NormalizePath(dirPath)

	if !strings.HasSuffix(dirPath, "/") {
		dirPath += "/"
	}

	return strings.HasPrefix(filePath, dirPath) || filePath == strings.TrimSuffix(dirPath, "/")
}

// GetRelativePaths returns all possible relative paths for a file
func (pu *PathUtils) GetRelativePaths(filePath string) []string {
	filePath = pu.NormalizePath(filePath)
	parts := strings.Split(filePath, "/")

	var paths []string
	for i := 0; i < len(parts); i++ {
		subPath := strings.Join(parts[i:], "/")
		if subPath != "" {
			paths = append(paths, subPath)
		}
	}

	return paths
}

func getUniqueOwners(owners []Owner) []string {
	unique := make(map[string]bool)
	for _, owner := range owners {
		unique[owner.Original] = true
	}

	result := make([]string, 0, len(unique))
	for owner := range unique {
		result = append(result, owner)
	}
	return result
}

func findMostUsedPattern(patternFileCount map[string]int) string {
	maxCount := 0
	mostUsed := ""
	for pattern, count := range patternFileCount {
		if count > maxCount {
			maxCount = count
			mostUsed = pattern
		}
	}
	return mostUsed
}

func findMostUsedCount(patternFileCount map[string]int) int {
	maxCount := 0
	for _, count := range patternFileCount {
		if count > maxCount {
			maxCount = count
		}
	}
	return maxCount
}
