package directives

import (
	_ "github.com/origadmin/abgen/testdata/fixtures/ent"
	_ "github.com/origadmin/abgen/testdata/fixtures/types"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixtures/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixtures/types,alias=types

// Phase for Multi-Source Package Conversions
// Tests: merging multiple source packages into single target package

//go:abgen:pair:packages="ent,types"
//go:abgen:pair:packages="types,ent"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="Multi"

// Expected scenario:
// 1. ent.User + types.Role → merged target structure
// 2. Multiple source types combined into unified conversion
// 3. Cross-package field mapping and type resolution

// This tests the ability to handle multiple input packages
// and merge them into coherent output structures.