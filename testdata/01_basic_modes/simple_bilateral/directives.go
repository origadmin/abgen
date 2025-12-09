package simple_bilateral

import (
	_ "github.com/origadmin/abgen/testdata/fixture/ent"
	_ "github.com/origadmin/abgen/testdata/fixture/types"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/types,alias=types

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
