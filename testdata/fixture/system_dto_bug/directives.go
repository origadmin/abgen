package system_dto_bug

import (
	"github.com/origadmin/abgen/testdata/fixture/system_dto_bug/ent"
	"github.com/origadmin/abgen/testdata/fixture/system_dto_bug/types"
)

//go:abgen:pair:packages="github.com/origadmin/abgen/testdata/fixture/system_dto_bug/ent,github.com/origadmin/abgen/testdata/fixture/system_dto_bug/types"
//go:abgen:convert:source:suffix=""
//go:abgen:convert:target:suffix="PB"
//go:abgen:convert:direction="both"

//go:abgen:convert="User,UserPB"
//go:abgen:convert:direction="both"
//go:abgen:convert:ignore="password,salt"
//go:abgen:convert:field="Gender:string:ConvertGender2String,string:Gender:ConvertString2Gender"
//go:abgen:convert:source:suffix=""
//go:abgen:convert:target:suffix="PB"
type User = ent.User
type UserPB = types.User
