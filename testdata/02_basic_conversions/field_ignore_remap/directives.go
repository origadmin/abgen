package field_ignore_remap

import (
	_ "github.com/origadmin/abgen/testdata/02_basic_conversions/field_ignore_remap/source"
	_ "github.com/origadmin/abgen/testdata/02_basic_conversions/field_ignore_remap/target"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/02_basic_conversions/field_ignore_remap/source,alias=source
//go:abgen:package:path=github.com/origadmin/abgen/testdata/02_basic_conversions/field_ignore_remap/target,alias=target

//go:abgen:convert="source.User,target.UserDTO,ignore=Password;CreatedAt,remap=Name:FullName;Email:UserEmail"
