package source

import "time"

// DepartmentEdges holds the relations between Department and other entities.
type DepartmentEdges struct {
	Users           []*User
	Positions       []*Position
	Children        []*Department
	Parent          *Department
	UserDepartments []*UserDepartment
}

type User struct {
	ID        int
	Username  string
	Age       int
	Gender    Gender
	Status    string
	CreatedAt time.Time
	Edges     UserEdges
}

type UserEdges struct {
	Roles []*Role
}

type Position struct {
	ID   int
	Name string
}

type Department struct {
	ID    int
	Name  string
	Edges DepartmentEdges
}

type UserDepartment struct {
	UserID       int
	DepartmentID int
}

type Role struct {
	ID   int
	Name string
}

type Gender int

const (
	GenderMale Gender = iota
	GenderFemale
)
