# 三方转换示例测试用例

## 测试目标
验证abgen在标准三方转换模式下的完整功能：源包(ent) → 中转包(当前包) → 目标包(types)

## 结构体定义

### 源包 (ent package)
```go
// ent.User
type User struct {
    ID        int                    // 对应目标: Id
    Username  string                 // 对应目标: Username
    Password  string                 // 忽略字段
    Salt      string                 // 忽略字段
    Age       int                    // 对应目标: Age
    Gender    ent.Gender            // 对应目标: Gender
    CreatedAt time.Time             // 忽略字段
    UpdatedAt time.Time             // 忽略字段
    Status    string                 // 对应目标: Status (string -> int32)
    RoleIDs   []int                 // 映射到: Edges.Roles
    Roles     []*ent.Role           // 忽略字段
    Edges     ent.ResourceEdges     // 部分使用
}

type Gender int
const (
    GenderMale Gender = iota
    GenderFemale
)

type ResourceEdges struct {
    Roles []*Role
}

type Role struct {
    ID   int
    Name string
}
```

### 目标包 (types package)
```go
// types.User
type User struct {
    Id        int
    Username  string
    Age       int
    Gender    types.Gender      // string enum
    Status    int32            // int32 enum
    CreatedAt string
    Edges     *types.Edges    // 嵌套结构体
}

type Gender string
const (
    GenderMale   Gender = "male"
    GenderFemale Gender = "female"
)

type Edges struct {
    Roles []*Role
}

type Role struct {
    Id   int
    Name string
}
```

## 指令解析
```go
// 1. 包别名定义
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/ent,alias=source
//go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/types,alias=target

// 2. 批量配对
//go:abgen:pair:packages="source,target"

// 3. 全局命名规则
//go:abgen:convert:target:suffix="PB"

// 4. 字段映射规则
//go:abgen:convert:remap="source.User#ID:Id"
//go:abgen:convert:remap="source.User#RoleIDs:Edges.Roles"
//go:abgen:convert:ignore="source.User#Password,Salt,CreatedAt,UpdatedAt"

// 5. 方向控制
//go:abgen:convert:direction="both"
```

## 预期生成的代码

### 类型别名
```go
// 源类型保持原名（因为没有定义source前后缀）
type User = ent.User
type Gender = ent.Gender
type Role = ent.Role
type ResourceEdges = ent.ResourceEdges

// 目标类型添加PB后缀
type UserPB = types.User
type GenderPB = types.Gender
type RolePB = types.Role
type EdgesPB = types.Edges
```

### 转换函数
```go
// 主要转换函数
func ConvertUserToUserPB(from *User) *UserPB {
    if from == nil {
        return nil
    }
    to := &UserPB{}
    to.Id = from.ID
    to.Username = from.Username
    to.Age = from.Age
    to.Gender = ConvertGenderToGenderPB(from.Gender)
    to.Status = ConvertStatusToStatusPB(from.Status)
    to.CreatedAt = "" // 需要自定义规则或忽略
    to.Edges = ConvertResourceEdgesToEdgesPB(from.Edges)
    return to
}

func ConvertUserPBToUser(from *UserPB) *User {
    if from == nil {
        return nil
    }
    to := &User{}
    to.ID = from.Id
    to.Username = from.Username
    to.Age = from.Age
    to.Gender = ConvertGenderPBToGender(from.Gender)
    to.Status = ConvertStatusPBToStatus(from.Status)
    // Password, Salt, CreatedAt, UpdatedAt 保持零值
    to.Edges = ConvertEdgesPBToResourceEdges(from.Edges)
    return to
}

// 枚举转换函数
func ConvertGenderToGenderPB(from ent.Gender) types.Gender {
    switch from {
    case ent.GenderMale:
        return types.GenderMale
    case ent.GenderFemale:
        return types.GenderFemale
    default:
        return ""
    }
}

func ConvertGenderPBToGender(from types.Gender) ent.Gender {
    switch from {
    case types.GenderMale:
        return ent.GenderMale
    case types.GenderFemale:
        return ent.GenderFemale
    default:
        return ent.Gender(0)
    }
}

// 字符串到枚举转换
func ConvertStatusToStatusPB(from string) int32 {
    switch from {
    case "active":
        return types.StatusActive
    case "inactive":
        return types.StatusInactive
    case "pending":
        return types.StatusPending
    default:
        return types.StatusInactive
    }
}

func ConvertStatusPBToStatus(from int32) string {
    switch from {
    case types.StatusActive:
        return "active"
    case types.StatusInactive:
        return "inactive"
    case types.StatusPending:
        return "pending"
    default:
        return "inactive"
    }
}

// 嵌套结构体转换
func ConvertResourceEdgesToEdgesPB(from *ResourceEdges) *EdgesPB {
    if from == nil {
        return nil
    }
    to := &EdgesPB{}
    to.Roles = ConvertRolesToRolePBs(from.Roles)
    return to
}

func ConvertEdgesPBToResourceEdges(from *EdgesPB) *ResourceEdges {
    if from == nil {
        return nil
    }
    to := &ResourceEdges{}
    to.Roles = ConvertRolePBsToRoles(from.Roles)
    return to
}

// 切片转换函数
func ConvertRolesToRolePBs(from []*Role) []*RolePB {
    if from == nil {
        return nil
    }
    to := make([]*RolePB, len(from))
    for i := range from {
        to[i] = ConvertRoleToRolePB(from[i])
    }
    return to
}

func ConvertRolePBsToRoles(from []*RolePB) []*Role {
    if from == nil {
        return nil
    }
    to := make([]*Role, len(from))
    for i := range from {
        to[i] = ConvertRolePBToRole(from[i])
    }
    return to
}

// 基础结构体转换
func ConvertRoleToRolePB(from *Role) *RolePB {
    if from == nil {
        return nil
    }
    to := &RolePB{}
    to.Id = from.ID
    to.Name = from.Name
    return to
}

func ConvertRolePBToRole(from *RolePB) *Role {
    if from == nil {
        return nil
    }
    to := &Role{}
    to.ID = from.Id
    to.Name = from.Name
    return to
}
```

## 测试验证点
1. ✅ 类型别名生成：源类型保持原名，目标类型添加PB后缀
2. ✅ 字段映射：ID -> Id, RoleIDs -> Edges.Roles
3. ✅ 字段忽略：Password, Salt, CreatedAt, UpdatedAt
4. ✅ 枚举转换：int enum -> string enum
5. ✅ 嵌套结构体：ResourceEdges -> Edges
6. ✅ 切片转换：[]*Role -> []*RolePB
7. ✅ 双向转换：direction="both" 生成的两个方向函数
8. ✅ nil检查：所有指针和切片类型都有nil检查

## 复杂性体现
- **三方转换**：涉及源、中转、目标三个包
- **类型差异**：相同概念在不同包中有不同定义（Gender, Role）
- **嵌套映射**：字段从顶层映射到嵌套结构体
- **混合转换**：基础类型、枚举、切片、嵌套结构体的混合转换