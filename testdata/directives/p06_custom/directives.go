package directives

import (
	_ "github.com/origadmin/abgen/testdata/fixture/ent"
	_ "github.com/origadmin/abgen/testdata/fixture/types"
)

// Phase 6: Custom Rule and Dummy File Generation
// This file tests the highest-priority 'rule' directive and the generation
// of a safe-to-edit 'abgen.custom.go' file for placeholder functions.

//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/ent,alias=ent
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/types,alias=pb

// The 'User' struct has a 'CreatedAt' field of type 'time.Time'.
// We want to convert this to a 'string' in the target.
// This is a complex conversion that abgen cannot handle automatically.
// We use a 'rule' to delegate this to a custom function.
//go:abgen:convert="ent.User,pb.User"
//go:abgen:convert:rule="source:time.Time,target:string,func:ConvertTimeToString"

// Expected outcome:
// 1. In the main `ConvertUserToUserPB` function, for the `CreatedAt` field,
//    abgen generates a call to `ConvertTimeToString(from.CreatedAt)`.
// 2. abgen creates a new file named 'abgen.custom.go' in this same directory.
// 3. This new file contains a placeholder (dummy) implementation for `ConvertTimeToString`:
//    func ConvertTimeToString(from *time.Time) *string {
//        // TODO: Implement this custom conversion.
//        panic("custom conversion not implemented: ConvertTimeToString")
//    }
// 4. The project should compile successfully immediately after generation.
