package types

type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
)

type User struct {
	Id        int
	Username  string
	Age       int
	Gender    Gender
	Status    int32
	CreatedAt string
	// Note: No Edges field here
}

type Resource struct {
	Id       int
	Name     string
	ParentId int
}

type Role struct {
	Id   int
	Name string
}

// Edges struct for nested field mapping in remap test
type Edges struct {
	Roles []*Role
}

const (
	StatusInactive int32 = 0
	StatusActive   int32 = 1
	StatusPending  int32 = 2
)
