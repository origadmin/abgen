package directives

import (
	_ "github.com/origadmin/abgen/testdata/fixtures/ent"
	_ "github.com/origadmin/abgen/testdata/fixtures/types"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixtures/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixtures/types,alias=types

// Phase for String to Int Enum Conversions
// Tests: string → int32 enum conversions
// This maps to the original p05_enum test case.

//go:abgen:pair:packages="ent,types"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="Enum"

// Expected:
// 1. String to int32 enum conversion with switch-case
// 2. Automatic enum constant discovery (Status_*, Gender_*)
// 3. Reverse conversion (int32 enum to string)
// 4. Default value handling for unknown enum values
// 5. Fallback logic for invalid conversions