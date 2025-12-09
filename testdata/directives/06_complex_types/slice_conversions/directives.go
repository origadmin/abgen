package directives

import (
	_ "github.com/origadmin/abgen/testdata/fixture/ent"
	_ "github.com/origadmin/abgen/testdata/fixture/types"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/types,alias=types

// Phase for Slice Conversions
// Tests: []Type ↔ []*Type conversions
// This maps to the original p04_slice test case.

//go:abgen:pair:packages="ent,types"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="Slice"

// Expected:
// 1. Slice of values to slice of pointers conversion
// 2. Slice of pointers to slice of values conversion
// 3. Nested slice conversions with element type transformations
// 4. Nil slice handling and empty slice handling
