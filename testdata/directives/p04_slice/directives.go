package directives

import (
	_ "github.com/origadmin/abgen/testdata/fixture/ent"
	_ "github.com/origadmin/abgen/testdata/fixture/typespb"
)

// Phase 4: Automatic Slice Conversion
// This file tests abgen's ability to automatically handle conversions for slice fields.

//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/typespb,alias=pb

// By pairing the packages, abgen will find both 'User' and 'Role' types.
// The 'User' struct contains a 'Roles' field which is a slice of 'Role' structs.
//go:abgen:pair:packages="ent,pb"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="PB"

// Expected outcome:
// 1. abgen identifies the need for `ent.User` -> `pb.User` conversion.
// 2. Inside this conversion, it sees the `Roles` field, which is `[]*ent.Role`.
// 3. It identifies that `[]*ent.Role` needs to be converted to `[]*pb.Role`.
// 4. It promotes the `*ent.Role` -> `*pb.Role` conversion to a first-class conversion task.
// 5. It generates a standard, public function `ConvertRoleToRolePB`.
// 6. It generates a standard, public helper function `ConvertRolesToRolePBs` (or similar plural form).
// 7. The implementation of `ConvertRolesToRolePBs` MUST call `ConvertRoleToRolePB` for each element.
// 8. The main `ConvertUserToUserPB` function will then call `ConvertRolesToRolePBs` for the `Roles` field.
