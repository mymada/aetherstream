package auth

import (
	"errors"
	"fmt"
	"strings"

	"github.com/labstack/echo/v4"
)

// Permission represents a fine-grained action in the system.
type Permission string

const (
	PermLibraryRead   Permission = "library:read"
	PermLibraryWrite  Permission = "library:write"
	PermLibraryDelete Permission = "library:delete"
	PermItemRead      Permission = "item:read"
	PermItemWrite     Permission = "item:write"
	PermItemDelete    Permission = "item:delete"
	PermUserRead      Permission = "user:read"
	PermUserWrite     Permission = "user:write"
	PermUserDelete    Permission = "user:delete"
	PermSettingsRead  Permission = "settings:read"
	PermSettingsWrite Permission = "settings:write"
	PermStreamRead    Permission = "stream:read"
	PermStreamWrite   Permission = "stream:write"
	PermAdmin         Permission = "admin:*"
)

// Role defines a named set of permissions.
type Role struct {
	Name        string       `json:"name"`
	Permissions []Permission `json:"permissions"`
}

// RBAC defines the default roles and their permissions.
var RBAC = map[string]Role{
	"admin": {
		Name:        "admin",
		Permissions: []Permission{PermAdmin},
	},
	"user": {
		Name: "user",
		Permissions: []Permission{
			PermLibraryRead,
			PermItemRead,
			PermItemWrite,
			PermStreamRead,
			PermStreamWrite,
			PermUserRead,
		},
	},
	"guest": {
		Name: "guest",
		Permissions: []Permission{
			PermLibraryRead,
			PermItemRead,
			PermStreamRead,
		},
	},
}

// HasPermission checks if a role has a specific permission.
func HasPermission(role string, perm Permission) bool {
	r, ok := RBAC[role]
	if !ok {
		return false
	}
	for _, p := range r.Permissions {
		if p == PermAdmin || p == perm {
			return true
		}
		// Support wildcards like "library:*"
		if strings.HasSuffix(string(p), ":*") {
			prefix := strings.TrimSuffix(string(p), ":*")
			if strings.HasPrefix(string(perm), prefix+":") {
				return true
			}
		}
	}
	return false
}

// RequirePermission returns Echo middleware that checks for a specific permission.
func getUserRole(c echo.Context) string {
	if user, ok := c.Get("user").(interface{ GetRole() string }); ok {
		return user.GetRole()
	}
	return ""
}

// RequirePermission returns Echo middleware that checks for a specific permission.
func RequirePermission(perm Permission) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role := getUserRole(c)
			if role == "" {
				return echo.NewHTTPError(401, "unauthorized")
			}
			if !HasPermission(role, perm) {
				return echo.NewHTTPError(403, fmt.Sprintf("missing permission: %s", perm))
			}
			return next(c)
		}
	}
}

// RequireAnyPermission returns Echo middleware that checks for any of the given permissions.
func RequireAnyPermission(perms ...Permission) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role := getUserRole(c)
			if role == "" {
				return echo.NewHTTPError(401, "unauthorized")
			}
			for _, perm := range perms {
				if HasPermission(role, perm) {
					return next(c)
				}
			}
			return echo.NewHTTPError(403, "missing required permissions")
		}
	}
}

// UserPermissions returns the list of permissions for a given role.
func UserPermissions(role string) ([]Permission, error) {
	r, ok := RBAC[role]
	if !ok {
		return nil, errors.New("unknown role")
	}
	return r.Permissions, nil
}

// ListRoles returns all defined role names.
func ListRoles() []string {
	roles := make([]string, 0, len(RBAC))
	for name := range RBAC {
		roles = append(roles, name)
	}
	return roles
}

// AddRole dynamically adds a custom role (useful for plugins/enterprise).
func AddRole(name string, perms []Permission) {
	RBAC[name] = Role{
		Name:        name,
		Permissions: perms,
	}
}

// RemoveRole removes a custom role. Built-in roles (admin, user, guest) cannot be removed.
func RemoveRole(name string) error {
	if name == "admin" || name == "user" || name == "guest" {
		return errors.New("cannot remove built-in role")
	}
	delete(RBAC, name)
	return nil
}
