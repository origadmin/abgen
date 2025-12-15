package target

import "time"

type UserDTO struct {
	ID        int
	FullName  string
	UserEmail string
	CreatedDate time.Time
	LastUpdate  time.Time
}
