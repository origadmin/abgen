package directives

import (
	_ "github.com/origadmin/abgen/testdata/fixture/ent"
	_ "github.com/origadmin/abgen/testdata/fixture/typespb"
)

// Phase 5: Automatic String-to-Enum Conversion
// This file tests abgen's ability to automatically generate conversion logic
// between string and enum-like integer types.

//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/typespb,alias=pb

// The 'User' struct has a 'Status' field, which is 'ent.Status' (a string)
// in the source and 'pb.Status' (an int32 enum) in the target.
// NO explicit 'rule' directive is provided.
//go:abgen:pair:packages="ent,pb"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="PB"

// Expected outcome:
// 1. abgen identifies the need for `ent.User` -> `pb.User` conversion.
// 2. Inside, it sees the `Status` field requires `ent.Status` (string) -> `pb.Status` (int32) conversion.
// 3. It promotes this to a first-class conversion task.
// 4. It generates a standard, public function `ConvertStatusToStatusPB` (or similar).
// 5. The body of this function MUST be a `switch` statement that maps string values to enum constants.
//    abgen should automatically discover constants like `pb.Status_Active`.
// 6. The main `ConvertUserToUserPB` function will then call `ConvertStatusToStatusPB` for the `Status` field.
// 7. The reverse conversion (`ConvertStatusPBToStatus`) should also be generated.
