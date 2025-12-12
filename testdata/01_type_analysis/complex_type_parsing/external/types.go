//go:build abgen
// +build abgen

package external

import "time"

// User represents a user model in an external package.
type User struct {
	ID        int
	Username  string
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}
