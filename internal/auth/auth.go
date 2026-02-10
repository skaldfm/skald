package auth

import (
	"context"

	"github.com/skaldfm/skald/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const userKey contextKey = "user"
const showIDsKey contextKey = "showIDs"

// HashPassword returns a bcrypt hash of the password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword compares a bcrypt hashed password with a plaintext password.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// WithUser stores a user in the request context.
func WithUser(ctx context.Context, u *models.User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

// UserFromContext retrieves the user from the request context.
func UserFromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(userKey).(*models.User)
	return u
}

// WithShowIDs stores accessible show IDs in the context.
func WithShowIDs(ctx context.Context, ids []int64) context.Context {
	return context.WithValue(ctx, showIDsKey, ids)
}

// AccessibleShowIDs returns the list of show IDs the user can access.
// Returns nil for admins (meaning all shows).
func AccessibleShowIDs(ctx context.Context) []int64 {
	ids, _ := ctx.Value(showIDsKey).([]int64)
	return ids
}

// IsAdmin returns true if the user has the admin role.
func IsAdmin(u *models.User) bool {
	return u != nil && u.Role == "admin"
}

// CanEdit returns true if the user can create/edit content (admin or editor).
func CanEdit(u *models.User) bool {
	return u != nil && (u.Role == "admin" || u.Role == "editor")
}

// CanAccessShow returns true if the user can view the given show.
func CanAccessShow(ctx context.Context, showID int64) bool {
	user := UserFromContext(ctx)
	if user == nil {
		return false
	}
	if IsAdmin(user) {
		return true
	}
	ids := AccessibleShowIDs(ctx)
	for _, id := range ids {
		if id == showID {
			return true
		}
	}
	return false
}
