package simple_bilateral

import (
	_ "github.com/origadmin/abgen/testdata/fixtures/ent"
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

// ConvertUserStatusToUserBilateralStatus is a custom conversion function stub.
// Please implement this function to complete the conversion.
func ConvertUserStatusToUserBilateralStatus(from string) int32 {
	// TODO: Implement this custom conversion
	panic("stub! not implemented")
}

// ConvertUserBilateralStatusToUserStatus is a custom conversion function stub.
// Please implement this function to complete the conversion.
func ConvertUserBilateralStatusToUserStatus(from int32) string {
	// TODO: Implement this custom conversion
	panic("stub! not implemented")
}
