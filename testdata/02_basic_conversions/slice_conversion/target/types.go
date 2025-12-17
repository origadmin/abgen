package target

// Item and Order are for the original, simple test case.
type Item struct {
	ID int
}

type Order struct {
	ID    int
	Items []Item
}

// --- Test Case: []Value -> []Value ---
type UserVV struct {
	Name string
}
type ContainerVV struct {
	Users []UserVV
}

// --- Test Case: []*Ptr -> []*Ptr ---
type UserPP struct {
	Name string
}
type ContainerPP struct {
	Users []*UserPP
}

// --- Test Case: *[]Value -> *[]Value ---
type UserPV struct {
	Name string
}
type ContainerPV struct {
	Users *[]UserPV
}

// --- Test Case: *[]*Ptr -> *[]*Ptr ---
type UserPPP struct {
	Name string
}
type ContainerPPP struct {
	Users *[]*UserPPP
}

// --- Test Case: []Value -> []*Ptr ---
type UserVP struct {
	Name string
}
type ContainerVP struct {
	Users []*UserVP // Target for []UserVP -> []*UserVP
}

// --- Test Case: []*Ptr -> []Value ---
type UserPV2 struct {
	Name string
}
type ContainerPV2 struct {
	Users []UserPV2 // Target for []*UserPV2 -> []UserPV2
}
