package directives

import (
	"github.com/origadmin/abgen/testdata/fixture/ent"
	"github.com/origadmin/abgen/testdata/fixture/types" // Corrected import alias
)

// --- Phase 1: Basic Struct-to-Struct Conversion ---
// This file tests the fundamental conversion capabilities of abgen
// with essential remap and ignore rules for the User type.

// 1. Conversion Binding Rule (Batch): Pair the packages to find matching types.
//    abgen should find 'User' in both packages and create a conversion task.
//go:abgen:pair:packages="github.com/origadmin/abgen/testdata/fixture/ent,github.com/origadmin/abgen/testdata/fixture/types" // Corrected target package path

// 2. Conversion Behavior Rules (Global):
//    - Apply a suffix to the target type's alias.
//    - Direction is not specified, so it defaults to "oneway".
//go:abgen:convert:target:suffix="PB"

// 3. Field Control Rule (Precise Remap):
//    - Remap 'ID' to 'Id' for the 'User' type.
//go:abgen:convert:remap="github.com/origadmin/abgen/testdata/fixture/ent.User#ID:Id"

// 4. Field Control Rule (Precise Ignore):
//    - Ignore fields from the source that do not exist in the target, or are handled in later phases.
//go:abgen:convert:ignore="github.com/origadmin/abgen/testdata/fixture/ent.User#Password,Salt,CreatedAt,UpdatedAt,Edges,Gender"

// Type aliases are defined here to be used in the generated code.
// The generator should create 'UserPB' based on the target suffix rule.
type User = ent.User
type UserPB = types.User // Corrected alias usage
