package codeowners

import (
	"fmt"
	"strings"
)

// parseOwner parses a single owner specification with enhanced validation
func (p *Parser) parseOwner(ownerStr string) (*Owner, error) {
	owner := &Owner{
		Original: ownerStr,
	}

	// Check for role (@@prefix)
	if p.roleRegex.MatchString(ownerStr) {
		owner.Type = OwnerTypeRole
		owner.IsRole = true
		owner.Name = strings.ToLower(ownerStr[2:]) // Remove @@ and normalize

		// Validate role
		validRoles := map[string]bool{
			"developer":   true,
			"developers":  true,
			"maintainer":  true,
			"maintainers": true,
			"owner":       true,
			"owners":      true,
		}

		if !validRoles[owner.Name] {
			return nil, fmt.Errorf("invalid role: %s", owner.Name)
		}

		return owner, nil
	}

	// Check for group or user (@prefix)
	if p.userOrGroupRegex.MatchString(ownerStr) {
		ownerName := ownerStr[1:] // Remove @ prefix

		// Check if it's a nested group (contains /)
		if strings.Contains(ownerName, "/") {
			owner.Type = OwnerTypeGroup
			owner.IsGroup = true
			owner.IsNested = true
		} else {
			owner.Type = OwnerTypeGroup
			owner.IsGroup = true
		}

		owner.Name = ownerName
		return owner, nil
	}

	// Check if it's an email address
	if p.emailRegex.MatchString(ownerStr) {
		owner.Type = OwnerTypeUser
		owner.IsEmail = true
		owner.Name = ownerStr
		return owner, nil
	}

	// Check for malformed owners (no @ symbol for non-email users)
	if !p.roleRegex.MatchString(ownerStr) && !p.userOrGroupRegex.MatchString(ownerStr) && !p.emailRegex.MatchString(ownerStr) {
		return nil, fmt.Errorf("malformed owner: %s", ownerStr)
	}

	// Default to username
	owner.Type = OwnerTypeUser
	owner.Name = ownerStr
	return owner, nil
}

// parseOwners parses owner specifications from a string with enhanced validation
func (p *Parser) parseOwners(ownersStr string) ([]Owner, error) {
	var owners []Owner
	var invalidOwners []string

	// Split by whitespace to get individual owners
	ownerParts := strings.Fields(ownersStr)

	for _, part := range ownerParts {
		owner, err := p.parseOwner(part)
		if err != nil {
			invalidOwners = append(invalidOwners, part)
			continue
		}
		owners = append(owners, *owner)
	}

	// Log invalid owners but don't fail the entire rule
	if len(invalidOwners) > 0 && p.strictValidation {
		return owners, fmt.Errorf("invalid owners ignored: %v", invalidOwners)
	}

	return owners, nil
}

// isOwnerAccessible checks if an owner is in the accessible owners list using the new structure
func (p *Parser) isOwnerAccessible(ownerStr string) bool {
	//fmt.Printf
	p.logger.Debug("Checking accessibility for owner", "owner", ownerStr)

	owner, err := p.parseOwner(ownerStr)
	if err != nil {
		p.logger.Debug("Parse error", "owner", ownerStr, "error", err)
		return false
	}

	p.logger.Debug("Parsed owner", "type", owner.Type, "isRole", owner.IsRole, "name", owner.Name)

	// Check accessibility based on owner type
	switch owner.Type {
	case OwnerTypeRole:
		if p.HasAccessibleOwners() {
			accessible := p.accessibleOwners.Roles[owner.Name]
			p.logger.Debug("Checking role in accessible roles", "role", owner.Name, "accessible", accessible)
			return accessible
		}
		// Roles are accessible by default if no accessible owners are configured
		p.logger.Debug("Role is accessible (no restrictions)", "role", ownerStr)
		return true

	case OwnerTypeUser:
		if owner.IsEmail {
			if p.HasAccessibleOwners() {
				accessible := p.accessibleOwners.Emails[owner.Name]
				p.logger.Debug("Checking email in accessible emails", "email", owner.Name, "accessible", accessible)
				return accessible
			}
		} else {
			if p.HasAccessibleOwners() {
				accessible := p.accessibleOwners.Users[owner.Name]
				p.logger.Debug("Checking user in accessible users", "user", owner.Name, "accessible", accessible)
				return accessible
			}
		}
		// Default behavior based on validation mode
		p.logger.Debug("No accessible users/emails configured, using validation mode", "strict", p.strictValidation)
		return !p.strictValidation

	case OwnerTypeGroup:
		if owner.IsEmail {
			if p.HasAccessibleOwners() {
				accessible := p.accessibleOwners.Emails[owner.Name]
				p.logger.Debug("Checking email in accessible emails", "email", owner.Name, "accessible", accessible)
				return accessible
			}
		} else {
			if p.HasAccessibleOwners() {
				accessible := p.accessibleOwners.Users[owner.Name]
				p.logger.Debug("Checking user in accessible users", "user", owner.Name, "accessible", accessible)
				return accessible
			}
		}
		if p.HasAccessibleOwners() {
			accessible := p.accessibleOwners.Groups[owner.Name]
			p.logger.Debug("Checking group in accessible groups", "group", owner.Name, "accessible", accessible)
			return accessible
		}
		// Default behavior based on validation mode
		p.logger.Debug("No accessible groups configured, using validation mode: strict=%v\n", p.strictValidation)
		return !p.strictValidation
	}

	// Default behavior based on validation mode
	p.logger.Debug("Unknown owner type, using validation mode: strict=%v\n", p.strictValidation)
	return !p.strictValidation
}

// SetAccessibleOwners sets the accessible owners with separated types
func (p *Parser) SetAccessibleOwners(accessibleOwners *AccessibleOwners) {
	p.accessibleOwners = accessibleOwners
}

// SetAccessibleOwnersFromStrings parses and sets accessible owners from string slices
func (p *Parser) SetAccessibleOwnersFromStrings(users, groups, roles, emails []string) {
	p.accessibleOwners = NewAccessibleOwners()

	// Add users (without @ prefix)
	for _, user := range users {
		cleanUser := strings.TrimPrefix(user, "@")
		p.accessibleOwners.Users[cleanUser] = true
	}

	// Add groups (without @ prefix)
	for _, group := range groups {
		cleanGroup := strings.TrimPrefix(group, "@")
		p.accessibleOwners.Groups[cleanGroup] = true
	}

	// Add roles (without @@ prefix)
	for _, role := range roles {
		cleanRole := strings.TrimPrefix(strings.TrimPrefix(role, "@@"), "@")
		p.accessibleOwners.Roles[cleanRole] = true
	}

	// Add emails (as-is)
	for _, email := range emails {
		p.accessibleOwners.Emails[email] = true
	}
}

// AddAccessibleUser adds a user to the accessible owners
func (p *Parser) AddAccessibleUser(username string) {
	cleanUser := strings.TrimPrefix(username, "@")
	p.accessibleOwners.Users[cleanUser] = true
}

// AddAccessibleGroup adds a group to the accessible owners
func (p *Parser) AddAccessibleGroup(groupname string) {
	cleanGroup := strings.TrimPrefix(groupname, "@")
	p.accessibleOwners.Groups[cleanGroup] = true
}

// AddAccessibleRole adds a role by access level (integer) and maps it to role names
func (p *Parser) AddAccessibleRole(accessLevel int) {
	p.accessibleOwners.RoleLevel = accessLevel

	// Map access level to role names and add them to accessible roles
	for roleName, level := range p.accessLevelMap {
		if level <= accessLevel {
			p.accessibleOwners.Roles[roleName] = true
		}
	}
}

// AddAccessibleRoleByName adds a specific role by name (for backward compatibility)
func (p *Parser) AddAccessibleRoleByName(rolename string) {
	cleanRole := strings.TrimPrefix(strings.TrimPrefix(rolename, "@@"), "@")
	p.accessibleOwners.Roles[cleanRole] = true
}

// AddAccessibleEmail adds an email to the accessible owners
func (p *Parser) AddAccessibleEmail(email string) {
	p.accessibleOwners.Emails[email] = true
}

// GetAccessibleOwners returns the current accessible owners
func (p *Parser) GetAccessibleOwners() *AccessibleOwners {
	return p.accessibleOwners
}

// IsAccessibleUser checks if a user is accessible
func (p *Parser) IsAccessibleUser(username string) bool {
	cleanUser := strings.TrimPrefix(username, "@")
	return p.accessibleOwners.Users[cleanUser]
}

// IsAccessibleGroup checks if a group is accessible
func (p *Parser) IsAccessibleGroup(groupname string) bool {
	cleanGroup := strings.TrimPrefix(groupname, "@")
	return p.accessibleOwners.Groups[cleanGroup]
}

// IsAccessibleRole checks if a role is accessible
func (p *Parser) IsAccessibleRole(rolename string) bool {
	cleanRole := strings.TrimPrefix(strings.TrimPrefix(rolename, "@@"), "@")
	return p.accessibleOwners.Roles[cleanRole]
}

// IsAccessibleEmail checks if an email is accessible
func (p *Parser) IsAccessibleEmail(email string) bool {
	return p.accessibleOwners.Emails[email]
}

// HasAccessibleOwners returns true if any accessible owners are configured
func (p *Parser) HasAccessibleOwners() bool {
	return len(p.accessibleOwners.Users) > 0 ||
		len(p.accessibleOwners.Groups) > 0 ||
		len(p.accessibleOwners.Roles) > 0 ||
		len(p.accessibleOwners.Emails) > 0
}
