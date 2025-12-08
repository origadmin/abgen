package bug_fix_001

import (
	"github.com/origadmin/abgen/testdata/fixture/ent"
	"github.com/origadmin/abgen/testdata/fixture/types"
)

// --- Directives Corrected According to the Final Design (NO PACKAGE ALIASES) ---

// 1. Pair packages for automatic type discovery ('User').
//    Using full package paths as instructed.
//go:abgen:pair:packages="github.com/origadmin/abgen/testdata/fixture/system_dto_bug/ent,github.com/origadmin/abgen/testdata/fixture/system_dto_bug/types"

// 2. Set global conversion rules.
//go:abgen:convert:target:suffix="PB"
//go:abgen:convert:direction="both"

// 3. Handle case-insensitive field mapping using 'remap'.
//    Using full type path as instructed.
//go:abgen:convert:remap="github.com/origadmin/abgen/testdata/fixture/system_dto_bug/ent.User#ID:Id"

// 4. Precisely ignore fields from the source that do not exist in the target.
//    Using full type path as instructed.
//go:abgen:convert:ignore="github.com/origadmin/abgen/testdata/fixture/system_dto_bug/ent.User#Password,Salt,CreatedAt,UpdatedAt,Edges"

// 5. The old ':field' rule for 'Gender' has been REMOVED.
//    We are now relying on abgen's built-in intelligence to automatically
//    detect the 'ent.Gender (int)' <-> 'types.Gender (string)' conversion
//    and generate the appropriate switch-case logic.

// --- Type Aliases for Local Context ---
// These are kept for compiler correctness and clarity within this file.
type User = ent.User
type UserPB = types.User
