package directives

import (
	_ "github.com/origadmin/abgen/testdata/fixture/ent"
	_ "github.com/origadmin/abgen/testdata/fixture/types"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/types,alias=types

// Phase for Multi-Target Package Conversions
// Tests: single source package to multiple target packages

//go:abgen:pair:packages="ent,types"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="Target"

// Expected scenario:
// 1. ent.User → types.User + types.UserSummary
// 2. Single source structure converted to multiple target structures
// 3. Different projections and field mappings for each target

// This tests the ability to generate multiple output formats
// from a single input structure.