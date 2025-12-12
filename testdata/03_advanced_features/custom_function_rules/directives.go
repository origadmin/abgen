package directives

import (
	_ "github.com/origadmin/abgen/testdata/fixtures/ent"
	_ "github.com/origadmin/abgen/testdata/fixtures/types"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixtures/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixtures/types,alias=types

// Phase for Custom Function Rules
// Tests: custom conversion function rules
// This maps to the original p06_custom test case.

//go:abgen:pair:packages="ent,types"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="Custom"

// Expected:
// 1. Custom function registration and usage
// 2. Cross-package custom function calls
// 3. Custom rule precedence over auto-generated conversions
// 4. Integration with standard conversion logic