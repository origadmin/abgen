package testdata

import (
	"github.com/origadmin/abgen/internal/testdata/ent"
	"github.com/origadmin/abgen/internal/testdata/typespb"
)

// This is a file-level directive group for package-to-package conversion.
//go:abgen:pair:packages="github.com/origadmin/abgen/internal/testdata/ent,github.com/origadmin/abgen/internal/testdata/typespb"
//go:abgen:convert:source:suffix=""
//go:abgen:convert:target:suffix="PB"
//go:abgen:convert:direction="both"
//go:abgen:convert:rule="source:ent.Status,target:string,func:ConvertStatusToString"
//go:abgen:convert:rule="source:string,target:ent.Status,func:ConvertString2Status"
//go:abgen:convert:ignore="Password,Salt"
//
//go:abgen:convert:remap="Roles:Edges.Roles"
//go:abgen:convert:remap="RoleIDs:Edges.Roles.ID"

// This is a type-level directive group for a specific alias pair.
//go:abgen:convert="User,UserPB"
//go:abgen:convert:ignore="CreatedAt,UpdatedAt"
type User = ent.User
type UserPB = typespb.User

// This is another type-level group to test directionality.
//go:abgen:convert="SingleDirection,SingleDirectionPB"
//go:abgen:convert:direction="from"
type SingleDirection = ent.User
type SingleDirectionPB = typespb.User
