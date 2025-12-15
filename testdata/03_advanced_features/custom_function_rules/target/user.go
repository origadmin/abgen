package target

// UserCustom represents a user in the target system.
type UserCustom struct {
	ID   int
	Name string
	// Status is a string-based status (e.g., "Active", "Inactive")
	Status string
}
