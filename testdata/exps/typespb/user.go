package typespb

type User struct {
	ID       int64
	Username string
	Password string
	Salt     string
	Age      int
	Status   string
	Roles    []*Role
	RoleIDs  []int64
}

type Role struct {
	ID   int64
	Name string
}
