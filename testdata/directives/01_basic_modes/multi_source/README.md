# Multi-Source Conversions

## 测试目标
验证多源包转换功能，包括：
- 多个源包合并到单一目标
- 跨包字段映射
- 类型冲突解决
- 复杂依赖关系处理

## 转换场景

### 1. 基础多源转换
```go
// 源包1 (ent)
type User struct {
    ID       int
    Username string
    RoleID   int
}

// 源包2 (types)
type Role struct {
    ID   int
    Name string
}

// 目标合并结构
type UserRole struct {
    UserID   int
    Username string
    RoleID   int
    RoleName string
}

// 预期生成转换函数
func ConvertUserAndRoleToUserRole(user *ent.User, role *types.Role) *UserRole {
    return &UserRole{
        UserID:   user.ID,
        Username: user.Username,
        RoleID:   user.RoleID,
        RoleName: role.Name,
    }
}
```

### 2. 多对多转换
```go
// 多个源 → 多个目标
// ent.User + types.Role → UserExtended + RoleExtended

func ConvertToMultiTargets(user *ent.User, role *types.Role) (*UserExtended, *RoleExtended) {
    userExt := ConvertUserToUserExtended(user)
    roleExt := ConvertRoleToRoleExtended(role)
    return userExt, roleExt
}
```

## 关键设计点

### 包依赖管理
- **依赖解析**：分析包之间的依赖关系
- **循环依赖**：检测和处理循环引用
- **导入顺序**：确保正确的包导入顺序

### 类型冲突解决
- **名称冲突**：重命名或别名处理
- **类型不兼容**：选择最佳转换路径
- **字段缺失**：提供默认值或跳过

### 指令组合
- **多包指令**：`go:abgen:package` 多个包定义
- **跨包配对**：`go:abgen:pair` 跨不同包
- **统一转换**：统一的转换规则和命名

## 指令设计

### 多包定义
```go
//go:abgen:package:path=ent,alias=source1
//go:abgen:package:path=types,alias=source2
//go:abgen:package:path=pb,alias=target

//go:abgen:pair:packages="source1,source2" -> "target"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="Merged"
```

### 复杂映射规则
```go
//go:abgen:field:source="source1.User.ID,target=User.UserID"
//go:abgen:field:source="source2.Role.Name,target=User.RoleName"
```

## 验证点
- [ ] 多包导入和依赖解析
- [ ] 跨包类型转换
- [ ] 名称冲突处理
- [ ] 转换函数生成（多参数）
- [ ] 复杂依赖关系处理
- [ ] 循环依赖检测和处理
- [ ] 性能优化（避免重复转换）