package directives

import (
	"github.com/origadmin/abgen/testdata/fixture/ent"
	"github.com/origadmin/abgen/testdata/fixture/types"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/types,alias=types

// Phase for Standard Trilateral Conversions  
// Tests: source + current + target package conversions
// This maps to the original example_trilateral test case.

//go:abgen:pair:packages="ent,types"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="Trilateral"

// Expected: classic three-way conversion pattern:
// Source Package (ent) → Current Package (directives) → Target Package (types)
// Following the abgen architecture of source → intermediate → target
type (
	Resource           = ent.Resource
	ResourceTrilateral = types.Resource
)
