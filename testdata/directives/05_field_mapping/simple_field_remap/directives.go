package directives

import (
	_ "github.com/origadmin/abgen/testdata/fixture/ent"
	_ "github.com/origadmin/abgen/testdata/fixture/types"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/types,alias=types

// Phase for Simple Field Remapping
// Tests: basic field name and path remapping
// This maps to the original p03_remap test case.

//go:abgen:pair:packages="ent,types"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="Remap"

// Expected:
// 1. Field name remapping (e.g., ID → Id, Username → username)
// 2. Simple path mapping for nested fields
// 3. Field type conversion combined with remapping