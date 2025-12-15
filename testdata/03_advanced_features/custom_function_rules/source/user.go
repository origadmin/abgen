package source

// User represents a user in the source system.
type User struct {
	ID   int
	Name string
	// Status is an integer-based enum (e.g., 1: Active, 2: Inactive)
	Status int
}
