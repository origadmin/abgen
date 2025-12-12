package directives

import (
	_ "github.com/origadmin/abgen/testdata/fixture/ent"
	_ "github.com/origadmin/abgen/testdata/fixture/types"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/types,alias=types

// Phase for Map Type Conversions
// Tests: map[K]V ↔ map[K]*V and map[K1]V ↔ map[K2]V conversions

//go:abgen:pair:packages="ent,types"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="Map"

// Expected conversions:
// ent.User.Metadata (map[string]string) -> types.User.Metadata (map[string]*string)
// ent.User.Tags (map[int]string) -> types.User.Tags (map[int32]*string) 
// ent.User.Scores (map[string]int) -> types.User.Scores (map[string]int64)

// Note: This test requires fixture types to have different map key/value types
// to test map conversion logic.