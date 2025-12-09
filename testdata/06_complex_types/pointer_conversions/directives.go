package directives

import (
	_ "github.com/origadmin/abgen/testdata/fixture/ent"
	_ "github.com/origadmin/abgen/testdata/fixture/types"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/types,alias=types

// Phase for Pointer and Value Type Conversions
// Tests: *Type ↔ Type conversions with nil safety

//go:abgen:pair:packages="ent,types"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="Ptr"

// Expected conversions:
// ent.User.ID (int) -> types.User.Id (*int) - value to pointer
// ent.User.Username (string) -> types.User.Username (*string) - value to pointer
// ent.User.Age (*int) -> types.User.Age (int) - pointer to value

// Note: This test requires fixture types to have different pointer/value combinations
// to test actual pointer conversion logic.