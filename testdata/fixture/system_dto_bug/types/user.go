package types

type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
)

type UserPB struct {
	ID       int
	Username string
	Age      int
	Gender   Gender
	// Note: No Edges field here
}
