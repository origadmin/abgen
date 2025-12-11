package source

// This file serves as a container for the abgen directives under test.
// The Go code itself is not relevant for this test case.

// 定义包别名 (ent 会覆盖)
//go:abgen:package:path=github.com/my/project/ent/v1,alias=ent
//go:abgen:package:path=github.com/my/project/ent/v2,alias=ent

//go:abgen:package:path=github.com/my/project/pb,alias=pb

// 定义全局命名规则
//go:abgen:convert:target:suffix=Model

// 定义包配对 (同时使用别名和完整路径)
//go:abgen:pair:packages=ent,pb
//go:abgen:pair:packages=github.com/another/pkg,pb

// 定义复杂的转换规则
//go:abgen:convert="source=ent.User,target=pb.User,direction=both,ignore=PasswordHash;Salt,remap=CreatedAt:CreatedTimestamp"

// 定义简单的转换规则
//go:abgen:convert="source=github.com/another/pkg.Data,target=pb.Data"
