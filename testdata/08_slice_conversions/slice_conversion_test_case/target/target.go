package target

// DepartmentEdges holds the relations between Department and other entities.
type DepartmentEdges struct {
	Users           []*User
	Positions       []*Position
	Children        []*Department
	Parent          *Department
	UserDepartments []*UserDepartment
}

type User struct {
	Id        int32
	Username  string
	Age       int32
	Gender    Gender
	Status    int32  // Assuming int32 for types.User status
	CreatedAt string // Assuming string for types.User CreatedAt
	Edges     UserEdges
}

type UserEdges struct {
	Roles []*Role
}

type Position struct {
	Id   int32
	Name string
}

type Department struct {
	Id    int32
	Name  string
	Edges DepartmentEdges
}

type UserDepartment struct {
	UserId       int32
	DepartmentId int32
}

type Role struct {
	Id   int32
	Name string
}

type Gender int32 // Assuming int32 for types.Gender

const (
	GenderMale   Gender = 0
	GenderFemale Gender = 1
)

const (
	StatusActive   int32 = 0
	StatusInactive int32 = 1
	StatusPending  int32 = 2
)
