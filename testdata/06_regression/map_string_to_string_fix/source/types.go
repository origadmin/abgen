package source

type MapToStringSource struct {
	ID       int
	Metadata map[string]string
	Tags     map[string]string
	Config   map[string]string
}