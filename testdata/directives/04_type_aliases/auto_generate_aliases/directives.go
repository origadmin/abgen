package directives

import (
	_ "github.com/origadmin/abgen/testdata/fixture/ent"
	_ "github.com/origadmin/abgen/testdata/fixture/types"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/types,alias=types

// Phase for Auto-Generated Type Aliases
// Tests: automatic type alias generation for source and target types
// This maps to the original p02_alias test case.

//go:abgen:pair:packages="ent,types"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="Alias"

// Expected: 
// 1. Auto-generate type aliases (e.g., type Gender = ent.Gender)
// 2. Auto-generate target aliases (e.g., type GenderPB = types.Gender)
// 3. Use aliases in generated conversion functions for cleaner code