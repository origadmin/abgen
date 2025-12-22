package directives

import (
	"github.com/origadmin/abgen/testdata/fixtures/ent"
	"github.com/origadmin/abgen/testdata/fixtures/types"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixtures/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixtures/types,alias=types

// --- Restoring Final Strategy ---
// With the core logic in generator.go now fixed, we use the correct directive
// format with fully qualified names (FQNs) to ensure the test passes.

//go:abgen:convert="source=github.com/origadmin/abgen/testdata/fixtures/ent.User,target=github.com/origadmin/abgen/testdata/fixtures/types.User"
//go:abgen:convert="source=github.com/origadmin/abgen/testdata/fixtures/ent.Resource,target=github.com/origadmin/abgen/testdata/fixtures/types.Resource"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="Trilateral"

// Local type aliases are kept for clarity but are not driving the conversion process directly.
type (
	User               = ent.User
	UserTrilateral     = types.User
	Resource           = ent.Resource
	ResourceTrilateral = types.Resource
)

// ConvertGenderTrilateralToGender handles the conversion between types.Gender (string) and ent.Gender (int).
func ConvertGenderTrilateralToGender(from types.Gender) ent.Gender {
	if from == types.GenderMale {
		return ent.GenderMale
	}
	return ent.GenderFemale
}

func ConvertGenderToGenderTrilateral(from ent.Gender) types.Gender {
	if from == ent.GenderMale {
		return types.GenderMale
	}
	return types.GenderFemale
}

// ConvertUserStatusToUserTrilateralStatus is a custom conversion function stub.
// Please implement this function to complete the conversion.
func ConvertUserStatusToUserTrilateralStatus(from string) int32 {
	// TODO: Implement this custom conversion
	panic("stub! not implemented")
}

// ConvertUserTrilateralStatusToUserStatus is a custom conversion function stub.
// Please implement this function to complete the conversion.
func ConvertUserTrilateralStatusToUserStatus(from int32) string {
	// TODO: Implement this custom conversion
	panic("stub! not implemented")
}
