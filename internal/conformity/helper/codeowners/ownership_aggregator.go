package codeowners

import (
	"fmt"
	"sort"
	"strings"
)

func aggregateFilesByOwnership(c *CODEOWNERSFile, paths []string) {
	// Map to group files by their ownership requirements
	ownershipGroups := make(map[string]*OwnershipGroup)

	for _, file := range paths {
		owners := c.GetOwnersForFile(file)

		for _, so := range owners {
			key := createOwnershipKey(so.Name, so.Owners, so.RequiredApprovals, so.MatchingPatterns)

			if group, exists := ownershipGroups[key]; exists {
				// Add file to existing group
				group.Files = append(group.Files, file)
			} else {
				// Create new ownership group
				ownershipGroups[key] = &OwnershipGroup{
					SectionName:       so.Name,
					Owners:            so.Owners,
					RequiredApprovals: so.RequiredApprovals,
					Files:             []string{file},
				}
			}
		}
	}

	// Print aggregated results
	fmt.Println("=== Aggregated File Ownership ===")
	for _, group := range ownershipGroups {
		fmt.Printf("Section: %s\n", group.SectionName)
		fmt.Printf("Owners (%d):\n", len(group.Owners))
		for _, owner := range group.Owners {
			fmt.Printf("  - %s (%s) [Email: %t, Role: %t, Group: %t, Valid: %t]\n",
				owner.Name, owner.Original, owner.IsEmail, owner.IsRole, owner.IsGroup, owner.IsValid)
		}
		fmt.Printf("Required approvals: %d\n", group.RequiredApprovals)
		fmt.Printf("Files (%d):\n", len(group.Files))

		// Sort files for consistent output
		sort.Strings(group.Files)
		for _, file := range group.Files {
			fmt.Printf("  - %s\n", file)
		}
		fmt.Println("---")
	}
}

// Alternative function that returns structured data instead of printing
func GetAggregatedOwnership(c *CODEOWNERSFile, paths []string) map[string]*OwnershipGroup {
	ownershipGroups := make(map[string]*OwnershipGroup)

	for _, file := range paths {
		owners := c.GetOwnersForFile(file)

		//fmt.Printf("file: %s, owners: %v", file, owners)

		for _, so := range owners {
			key := createOwnershipKey(so.Name, so.Owners, so.RequiredApprovals, so.MatchingPatterns)

			if group, exists := ownershipGroups[key]; exists {
				group.Files = append(group.Files, file)
			} else {
				ownershipGroups[key] = &OwnershipGroup{
					SectionName:       so.Name,
					Owners:            so.Owners,
					RequiredApprovals: so.RequiredApprovals,
					Files:             []string{file},
					MatchingPattern:   so.MatchingPatterns,
				}
			}
		}
	}

	// Sort files in each group
	for _, group := range ownershipGroups {
		sort.Strings(group.Files)
	}

	return ownershipGroups
}

// createOwnershipKey creates a unique key for grouping files with same ownership
func createOwnershipKey(sectionName string, owners []Owner, requiredApprovals int, matchingPatter []MatchingPattern) string {
	// Create keys for each owner and sort them to ensure consistent grouping
	ownerKeys := make([]string, len(owners))
	for i, owner := range owners {
		ownerKeys[i] = createOwnerKey(owner)
	}
	sort.Strings(ownerKeys)

	return fmt.Sprintf("%s|%s|%d", sectionName, strings.Join(ownerKeys, "||"), requiredApprovals)
}

// createOwnerKey creates a unique string representation of an owner for sorting/comparison
func createOwnerKey(owner Owner) string {
	return fmt.Sprintf("%s|%s|%t|%t|%t|%t|%t",
		owner.Name, owner.Original, owner.IsEmail, owner.IsRole,
		owner.IsGroup, owner.IsNested, owner.IsValid)
}
