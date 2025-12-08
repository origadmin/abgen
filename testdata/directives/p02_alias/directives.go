package directives

import (
	"github.com/origadmin/abgen/testdata/fixture/ent"
	"github.com/origadmin/abgen/testdata/fixture/types"
)

// Phase 2: Alias-First Naming Convention
// This file tests that abgen prioritizes local type aliases when naming conversion functions.

// Define local aliases for source and target types.
type UserEntity = ent.User
type UserProto = types.User

// Bind the conversion directly using the aliases.
//go:abgen:convert="UserEntity,UserProto"
//go:abgen:convert:direction="both"

// Expected outcome:
// The generator should find the local aliases 'UserEntity' and 'UserProto'.
// It should generate functions named 'ConvertUserEntityToUserProto' and 'ConvertUserProtoToUserEntity'.
// This proves that the "local alias" naming rule takes precedence over the global Prefix/Suffix rules.
