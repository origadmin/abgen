package directives

import (
	_ "github.com/origadmin/abgen/testdata/fixtures/ent"
	_ "github.com/origadmin/abgen/testdata/fixtures/types"
)

// 三方转换示例：源包(ent) → 中转包(当前包) → 目标包(types)

// 1. 定义包别名，简化后续指令
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixtures/ent,alias=source
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixtures/types,alias=target

// 2. 批量配对源包和目标包
//    abgen会在source和target包中查找同名类型
//go:abgen:pair:packages="source,target"

// 3. 全局命名规则：为目标类型添加PB后缀
//go:abgen:convert:target:suffix="PB"

// 4. 字段映射规则：处理源和目标之间的字段差异
//go:abgen:convert:remap="source.User#ID:Id"           // ID -> Id
//go:abgen:convert:remap="source.User#RoleIDs:Edges.Roles" // []int -> []*Role
//go:abgen:convert:ignore="source.User#Password,Salt,CreatedAt,UpdatedAt" // 忽略字段

// 5. 方向控制：双向转换
//go:abgen:convert:direction="both"

// 预期结果：
// 1. 生成 User -> UserPB 转换函数
// 2. 源类型保持原名（因为没有定义source前后缀）
// 3. 目标类型添加PB后缀
// 4. 处理嵌套结构体转换