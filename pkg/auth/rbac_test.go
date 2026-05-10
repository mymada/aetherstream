package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasPermission(t *testing.T) {
	assert.True(t, HasPermission("admin", PermLibraryRead))
	assert.True(t, HasPermission("admin", PermAdmin))
	assert.True(t, HasPermission("user", PermLibraryRead))
	assert.True(t, HasPermission("user", PermItemRead))
	assert.False(t, HasPermission("user", PermLibraryDelete))
	assert.True(t, HasPermission("guest", PermStreamRead))
	assert.False(t, HasPermission("guest", PermItemWrite))
	assert.False(t, HasPermission("unknown", PermLibraryRead))
}

func TestUserPermissions(t *testing.T) {
	perms, err := UserPermissions("user")
	assert.NoError(t, err)
	assert.NotEmpty(t, perms)

	_, err = UserPermissions("nonexistent")
	assert.Error(t, err)
}

func TestListRoles(t *testing.T) {
	roles := ListRoles()
	assert.NotEmpty(t, roles)
	found := false
	for _, r := range roles {
		if r == "admin" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestAddAndRemoveRole(t *testing.T) {
	AddRole("moderator", []Permission{PermLibraryRead, PermItemWrite, PermUserRead})
	assert.True(t, HasPermission("moderator", PermItemWrite))

	err := RemoveRole("moderator")
	assert.NoError(t, err)
	assert.False(t, HasPermission("moderator", PermItemWrite))

	err = RemoveRole("admin")
	assert.Error(t, err)
}

func TestPermissionWildcard(t *testing.T) {
	AddRole("editor", []Permission{"library:*", "item:read"})
	assert.True(t, HasPermission("editor", PermLibraryRead))
	assert.True(t, HasPermission("editor", PermLibraryWrite))
	assert.True(t, HasPermission("editor", PermLibraryDelete))
	assert.True(t, HasPermission("editor", PermItemRead))
	assert.False(t, HasPermission("editor", PermItemWrite))
}
