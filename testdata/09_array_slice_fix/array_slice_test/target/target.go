package target

// Department represents a department entity in protobuf format
type Department struct {
	Id          int32
	Name        string
	Description string
	CreatedAt   string // time.Time -> string
}

// User represents a user entity in protobuf format
type User struct {
	Id          int32
	Username    string
	Age         int32
	Gender      Gender
	Status      int32  // string -> int32
	CreatedAt   string // time.Time -> string
	Departments []*Department
}

type Gender int32

const (
	GenderMale   Gender = 0
	GenderFemale Gender = 1
)
