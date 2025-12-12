package simple_struct

//go:abgen:package:path=github.com/origadmin/abgen/testdata/02_basic_conversions/simple_struct/source,alias=source
//go:abgen:package:path=github.com/origadmin/abgen/testdata/02_basic_conversions/simple_struct/target,alias=target

//go:abgen:convert="source=source.User,target=target.UserDTO,remap=Name:UserName"
