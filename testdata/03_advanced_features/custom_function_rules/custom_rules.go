package directives

//go:abgen:convert:rule="source:builtin.int,target:builtin.string,func:IntStatusToString"
//go:abgen:convert:rule="source:builtin.string,target:builtin.int,func:StringStatusToInt"

// IntStatusToString is a custom function to convert an integer status to a string.
func IntStatusToString(status int) string {
	switch status {
	case 1:
		return "Active"
	case 2:
		return "Inactive"
	default:
		return "Unknown"
	}
}

// StringStatusToInt is a custom function to convert a string status to an integer.
func StringStatusToInt(status string) int {
	switch status {
	case "Active":
		return 1
	case "Inactive":
		return 2
	default:
		return 0
	}
}
