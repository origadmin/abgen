package directives

import (
	_ "github.com/origadmin/abgen/testdata/fixture/ent"
	_ "github.com/origadmin/abgen/testdata/fixture/typespb"
)

// Phase 3: Field Path Remapping
// This file tests the advanced 'remap' functionality with nested field paths.

//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/typespb,alias=pb

// Test remap from a simple field to a nested field path.
//go:abgen:convert="ent.User,pb.User"
//go:abgen:convert:remap="ent.User#RoleIDs:Edges.Roles" // Map []int to []*Role
//go:abgen:convert:direction="both"

// Expected outcome:
// When converting from ent.User to pb.User, the generated code should be:
//   to.Edges.Roles = ConvertIntSliceToRoleSlice(from.RoleIDs) // (or similar)
// When converting from pb.User to ent.User, the generated code should be:
//   to.RoleIDs = ConvertRoleSliceToIntSlice(from.Edges.Roles)
// This tests the parser's ability to handle dot-separated paths and the generator's
// ability to produce code for nested assignments, including nil checks.
