package directives

import (
	_ "github.com/origadmin/abgen/testdata/fixture/ent"
	_ "github.com/origadmin/abgen/testdata/fixture/types"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/types,alias=types

// Phase for Numeric Type Conversions
// Tests: int ↔ int32 ↔ int64 ↔ float32 ↔ float64 conversions

//go:abgen:pair:packages="ent,types"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="Numeric"

// Expected conversions:
// ent.User.Age (int) -> types.User.Age (int32) - width conversion
// ent.User.Score (int) -> types.User.Score (float64) - int to float
// ent.User.Rating (float32) -> types.User.Rating (float64) - precision upgrade
// ent.User.Balance (float64) -> types.User.Balance (float32) - precision downgrade

// Note: This test requires fixture types to have different numeric types
// to test actual numeric conversion logic including overflow/underflow.