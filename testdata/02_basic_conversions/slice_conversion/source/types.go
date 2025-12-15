package source

type Item struct {
	ID int
}

type Order struct {
	ID    int
	Items []Item
}
