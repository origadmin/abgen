package source

import "time"

// Department represents a department entity
type Department struct {
	ID          int
	Name        string
	Description string
	CreatedAt   time.Time
}

// User represents a user entity
type User struct {
	ID         int
	Username   string
	Age        int
	Gender     Gender
	Status     string
	CreatedAt  time.Time
	Department *Department
}

type Gender int

const (
	GenderMale Gender = iota
	GenderFemale
)
