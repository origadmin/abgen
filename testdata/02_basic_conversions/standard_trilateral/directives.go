package directives

import (
	"time"

	"github.com/origadmin/abgen/testdata/fixtures/ent"
	"github.com/origadmin/abgen/testdata/fixtures/types"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixtures/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixtures/types,alias=types

// --- Restoring Final Strategy ---
// With the panic in generator.go fixed, we revert to the correct strategy
// of using fully qualified names (FQNs) in the `convert` directives.

//go:abgen:convert="source=github.com/origadmin/abgen/testdata/fixtures/ent.User,target=github.com/origadmin/abgen/testdata/fixtures/types.User"
//go:abgen:convert="source=github.com/origadmin/abgen/testdata/fixtures/ent.Resource,target=github.com/origadmin/abgen/testdata/fixtures/types.Resource"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="Trilateral"

// Custom conversion rules for fields where types are fundamentally different.
// These rules are now expected to work because the parent conversion is correctly specified.
//go:abgen:convert:rule="source:ent.Gender,target:types.Gender,func:ConvertGender"
//go:abgen:convert:rule="source:string,target:int32,func:ConvertStatus"

// Local type aliases are kept for clarity but are not driving the conversion process directly.
type (
	User               = ent.User
	UserTrilateral     = types.User
	Resource           = ent.Resource
	ResourceTrilateral = types.Resource
)

// ConvertGender handles the conversion between ent.Gender (int) and types.Gender (string).
func ConvertGender(from ent.Gender) types.Gender {
	if from == ent.GenderMale {
		return types.GenderMale
	}
	return types.GenderFemale
}

// ConvertStatus handles the conversion between a status string and a status int32.
func ConvertStatus(from string) int32 {
	switch from {
	case "active":
		return types.StatusActive
	case "inactive":
		return types.StatusInactive
	default:
		return types.StatusPending
	}
}

// ConvertTimeToString is an example of a function that abgen should infer automatically.
// It is included here for completeness but is not strictly required by a custom rule.
func ConvertTimeToString(from time.Time) string {
	return from.Format(time.RFC3339)
}
