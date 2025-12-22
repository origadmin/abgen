package simple_bilateral

import (
	"github.com/origadmin/abgen/testdata/fixtures/ent"
	_ "github.com/origadmin/abgen/testdata/fixtures/ent"
	"github.com/origadmin/abgen/testdata/fixtures/types"
	_ "github.com/origadmin/abgen/testdata/fixtures/types"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixtures/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixtures/types,alias=types

// Phase for Simple Bilateral Conversions
// Tests: current package + external package conversions
// This maps to the original p01_basic test case.

//go:abgen:pair:packages="ent,types"
//go:abgen:convert="ent.User,types.User"
//go:abgen:convert="ent.Resource,ResourceBilateral"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="Bilateral"

// Expected: standard ent.User ↔ types.User conversion
// with all basic type mappings and field copying.

func ConvertUserStatusToUserBilateralStatus(status string) int32 {
	// TODO: implement
	return 0
}

func ConvertUserBilateralStatusToUserStatus(status int32) string {
	// TODO: implement
	return ""
}

func ConvertGenderToGenderBilateral(gender ent.Gender) types.Gender {
	// TODO: implement
	return types.Gender(0)
}

func ConvertGenderBilateralToGender(gender types.Gender) ent.Gender {
	// TODO: implement
	return ent.Gender(0)
}
