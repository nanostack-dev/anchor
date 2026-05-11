package auth

import (
	"time"

	"github.com/nanostack-dev/shared/toolkit"
)

type User struct {
	ID             string
	ExternalID     *string
	Name           string
	Email          string
	HashedPassword string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// GenerateID sets the auth user's ID to a new prefixed KSUID.
func (u *User) GenerateID() {
	u.ID = toolkit.NewID("user")
}
