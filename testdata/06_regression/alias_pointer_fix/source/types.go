package source

type MyPointerField string

type User struct {
	ID        string
	Name      string
	Email     *string
	Status    MyPointerField
	Addresses []*Address
}

type Address struct {
	Street string
	City   string
}
