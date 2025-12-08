package types

type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
)

type User struct {
	Id       int
	Username string
	Age      int
	Gender   Gender
	// Note: No Edges field here
}

type Resource struct {
	Id       int
	Name     string
	ParentId int
}
