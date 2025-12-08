//go:build abgen_p01_basic

package directives

import (
	_ "github.com/origadmin/abgen/internal/testdata/fixture/ent"
	_ "github.com/origadmin/abgen/internal/testdata/fixture/typespb"
)

// Phase 1: Basic Struct-to-Struct Conversion
// This file tests the fundamental conversion capabilities of abgen.

// 1. Package Definition Rules: Define internal aliases for source and target packages.
//go:abgen:package:path=github.com/origadmin/abgen/internal/testdata/fixture/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/internal/testdata/fixture/typespb,alias=pb

// 2. Conversion Binding Rule (Batch): Pair the packages to find matching types.
// abgen should find 'User' in both packages and create a conversion task.
//go:abgen:pair:packages="ent,pb"

// 3. Conversion Behavior Rules (Global):
//    - Apply a suffix to the target type's alias.
//    - Enable bidirectional conversion for all pairs found by the above rule.
//go:abgen:convert:target:suffix="PB"
//go:abgen:convert:direction="both"

// 4. Field Control Rule (Precise):
//    - Ignore 'Password' and 'Salt' fields specifically for the 'User' type from the 'ent' package.
//    - This tests the new precise ignore syntax.
//go:abgen:convert:ignore="ent.User#Password,Salt"

// Type aliases are defined here to be used in the generated code.
// The generator should create 'UserPB' based on the target suffix rule.
type User = ent.User
type UserPB = pb.User
