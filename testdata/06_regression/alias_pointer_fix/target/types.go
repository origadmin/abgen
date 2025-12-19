package target

type MyPointerField string

type User struct {
	UserID    string
	UserName  string
	UserEmail *string
	UserStatus MyPointerField
	UserAddresses []*Address
}

type Address struct {
	StreetName string
	CityName   string
}
