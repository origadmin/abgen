# abgen 指令参考手册

本文档是 `abgen` 工具的官方指令参考手册，旨在帮助用户理解和有效使用 `abgen` 的各项功能。

## 快速参考 (Cheat Sheet)

| 功能 | 指令 | 示例 |
|:---|:---|:---|
| **定义包别名** | `//go:abgen:package:path` | `//go:abgen:package:path=github.com/my/project/ent,alias=ent_pkg` |
| **批量包配对** | `//go:abgen:pair:packages` | `//go:abgen:pair:packages="ent_pkg,pb_pkg"` |
| **单个类型配对** | `//go:abgen:convert` | `//go:abgen:convert="MyEntity,MyProto"` |
| **忽略字段** | `//go:abgen:convert:ignore` | `//go:abgen:convert:ignore="MyEntity#Password,Salt"` |
| **重命名字段** | `//go:abgen:convert:remap` | `//go:abgen:convert:remap="MyEntity#CreatedAt:CreatedTime"` |
| **自定义函数** | `//go:abgen:convert:rule` | `//go:abgen:convert:rule="source:builtin.int,target:builtin.string,func:IntToString"` |

---

## 核心概念

- **指令 (Directive)**: 在 Go 源码中以 `//go:abgen:` 开头的特殊注释，是用户与 `abgen` 交互的主要方式。`abgen` 会解析这些指令，并将其转化为具体的内部**规则**。
- **规则 (Rule)**: 从指令中解析出的具体操作指令。`abgen` 运行时遵循规则来生成代码。所有规则会被汇集到一个全局集合中，**后解析的规则会覆盖先解析的规则**。

---

## 指令详解 (Directives Reference)

### 1. 包与转换配对 (Packages & Pairing)

这类指令用于定义 `abgen` 需要处理的包以及哪些类型之间需要转换。

#### `//go:abgen:package:path`
定义一个 Go 包及其在 `abgen` 内部使用的别名。这个别名是后续指令引用该包的快捷方式。

- **格式**: `//go:abgen:package:path=<完整包导入路径>[,alias=<别名>]`
- **示例**:
  ```go
  //go:abgen:package:path=github.com/my/project/ent,alias=ent_source
  //go:abgen:package:path=github.com/my/project/pb,alias=pb_target
  ```
- **注意**: 此别名是 `abgen` 内部使用的，与 Go 的 `import` 别名无关。

#### `//go:abgen:pair:packages`
批量声明两个包之间需要进行转换。`abgen` 将自动查找这两个包中所有**名称相同**的类型，并为它们建立转换关系。

- **格式**: `//go:abgen:pair:packages="<源包别名>,<目标包别名>"`
- **示例**:
  ```go
  //go:abgen:pair:packages="ent_source,pb_target"
  ```

#### `//go:abgen:convert`
明确指定某两个类型之间需要进行转换。

- **格式**: `//go:abgen:convert="<源类型引用>,<目标类型引用>[,ignore=<字段1>[,<字段2>...]][,remap=<源字段路径>:<目标字段路径>...]"`
- **参数说明**:
  - `<源类型引用>`: 源类型的 Go 引用（必填）。
  - `<目标类型引用>`: 目标类型的 Go 引用（必填）。
  - `ignore`: 可选参数，指定在该转换中需要忽略的一个或多个字段。多个字段用逗号 `,` 分隔。
  - `remap`: 可选参数，指定字段重映射规则。
- **说明**: `<类型引用>` 可以是 Go 的类型别名（如 `UserEntity`）或全限定名。
- **示例**:
  ```go
  //go:abgen:convert="UserEntity,UserProto,ignore=Password;Salt"
  //go:abgen:convert="OrderModel,OrderDTO,remap=line_items:Items;customer_info:Customer"
  ```
### 2. 命名与别名控制 (Naming & Aliasing)

这类指令用于控制生成代码中的命名风格，包括函数名、类型别名等。

#### `//go:abgen:convert:(source|target):(prefix|suffix)`
为源或目标类型生成的名称添加全局的前缀或后缀。这会影响转换函数名和生成的类型别名。

- **格式**:
  - `//go:abgen:convert:source:prefix=<前缀>`
  - `//go:abgen:convert:source:suffix=<后缀>`
  - `//go:abgen:convert:target:prefix=<前缀>`
  - `//go:abgen:convert:target:suffix=<后缀>`
- **示例**:
  ```go
  //go:abgen:convert:source:suffix="Ent"  // 源类型别名加 "Ent" 后缀, e.g., UserEnt
  //go:abgen:convert:target:suffix="PB"   // 目标类型别名加 "PB" 后缀, e.g., UserPB
  ```

#### `//go:abgen:convert:alias:generate`
全局控制是否为转换中涉及的外部类型自动生成本地 `type` 别名。

- **格式**: `//go:abgen:convert:alias:generate=<true|false>`
- **默认值**: `true`。
- **当为 `true`**: `abgen` 会生成如 `type UserPB = pb.User` 的别名。
- **当为 `false`**: `abgen` 不会生成别名，并在代码中直接使用 `pb.User`。

#### 命名规则总结

| 元素 | 规则 | 示例 | 备注 |
|---|---|---|---|
| **结构体转换函数** | `Convert` + `[源类型名]` + `To` + `[目标类型名]` | `ConvertUserEntToUserPB` | 类型名根据前缀/后缀规则生成。 |
| **源/目标类型名** | `[前缀]` + `[原始名]` + `[后缀]` | `UserEnt`, `UserPB` | 1. 优先使用用户定义的 `prefix/suffix`。<br>2. 若无定义且与另一方重名，自动加 `Source/Target` 后缀。<br>3. 否则，使用原始名。 |

#### 切片类型别名命名 (Slice Type Alias Naming)
当 `abgen` 为切片类型（例如 `[]*pb.User`）生成别名时，它会采用简单的复数化规则：直接在**元素类型的最终别名**后添加 `s`。

**规则**: `[元素类型最终别名]` + `s`

**示例**:
- 如果 `pb.User` 的最终别名是 `UserPB`，那么 `[]*pb.User` 的别名将是 `UserPBs`。
- 如果 `ent.User` 的最终别名是 `UserSource`，那么 `[]*ent.User` 的别名将是 `UserSources`。

这种简化的方法确保了命名的一致性和可预测性。

#### 字段级转换函数命名 (Field-Level Conversion Function Naming)
当两个结构体的字段类型不兼容且无法直接转换时（例如 `string` ↔ `int`），`abgen` 会采取一种更通用和可复用的方法：

它会生成一个基于**字段类型**而不是字段名称的函数调用，并为其创建一个**桩函数 (Stub Function)**。

**命名规则**: `Convert[源字段类型名]To[目标字段类型名]`

**示例**:
- 一个 `string` 类型的字段需要转换为 `int`。
- `abgen` 会在转换代码中插入一个对 `ConvertStringToInt(...)` 函数的调用。
- 同时, `abgen` 会在 `custom.go` 文件中生成如下的桩函数:
  ```go
  // ConvertStringToInt converts from string to int.
  // Please implement this function.
  func ConvertStringToInt(from string) int {
      // TODO: Implement this custom conversion.
      panic("custom conversion not implemented for string to int")
  }
  ```
- **用户只需实现这一个函数**，所有在项目中遇到的 `string` 到 `int` 的不兼容字段转换都会自动使用此实现。

### 3. 转换行为控制 (Conversion Behavior)

这类指令用于精细化控制字段级别的转换逻辑。

#### `//go:abgen:convert:direction`
为转换对指定转换方向。

- **格式**: `//go:abgen:convert:direction=<oneway|both>`
- **默认值**: `both` (双向转换)。
- **`oneway`**: 只生成从源到目标的单向转换函数。

#### `//go:abgen:convert:ignore`
在特定类型的转换中忽略一个或多个字段。

- **格式**: `//go:abgen:convert:ignore="<类型引用>#<字段1>[,<字段2>...][;<类型2>#<字段3>...]"`
- **示例**:
  ```go
  // 忽略 UserEntity 中的 Password 和 Salt 字段
  //go:abgen:convert:ignore="UserEntity#Password,Salt"
  ```

#### `//go:abgen:convert:remap`
将源结构体中的一个字段值映射到目标结构体中一个不同名的字段。

- **格式**: `//go:abgen:convert:remap="<类型引用>#<源字段路径>:<目标字段路径>"`
- **说明**: 字段路径支持使用 `.` 访问内嵌结构体的字段。
- **示例**:
  ```go
  // 将 A 的 TypeIDs 字段映射到目标结构体的 Type.ID 字段
  //go:abgen:convert:remap="A#TypeIDs:Type.ID"
  ```

### 4. 高级规则 (Advanced Rules)

#### `//go:abgen:convert:rule`
使用用户自定义的函数来完全接管两个类型之间的转换，这是**最高优先级**的规则。

- **格式**: `//go:abgen:convert:rule="source:<源类型>,target:<目标类型>,func:<自定义函数名>"`
- **说明**:
  - 用于 `abgen` 无法自动处理的复杂情况，例如 `string` 与 `int` 之间的枚举转换。
  - `<源类型>` 和 `<目标类型>` 可以是 `builtin.string`, `builtin.int` 等内置类型。
  - `<自定义函数名>` 必须是当前包内可见的函数。`abgen` 会在 `custom.gen.go` 中生成函数桩。
- **示例**:
  ```go
  //go:abgen:convert:rule="source:builtin.int,target:builtin.string,func:IntStatusToString"
  //go:abgen:convert:rule="source:builtin.string,target:builtin.int,func:StringStatusToInt"
  ```

#### 规则优先级

| 优先级 | 规则类型 | 指令 | 说明 |
|:---:|:---|:---|:---|
| 1 (最高) | **自定义函数** | `convert:rule` | 完全覆盖，使用用户指定的函数。 |
| 2 | **字段重映射** | `convert:remap` | 覆盖单个字段的映射关系。 |
| 3 | **忽略字段** | `convert:ignore` | 排除特定字段不参与转换。 |
| 4 | **全局命名** | `source:suffix`, etc. | 影响函数和类型的命名。 |
| 5 (最低) | **自动转换** | (无) | `abgen` 的默认同名/同类型转换。 |

---

## 常见问题与最佳实践 (FAQ & Best Practices)

### 常见问题
- **Q: 为什么我的自定义函数没有被调用？**
  - **A:** 请检查：1. 规则的 `source` 和 `target` 类型是否正确；2. `func` 名称是否与包内函数名一致；3. 自定义函数是否为公开的（首字母大写）。
- **Q: 如何处理大型结构体的性能？**
  - **A:** 使用 `ignore` 指令排除不必要的字段；对于大量数据转换，考虑在业务代码中并行处理。
- **Q: 如何调试转换逻辑？**
  - **A:** 为关键的自定义转换函数编写单元测试；检查 `abgen` 在控制台输出的日志。

### 最佳实践
- **项目组织**:
  ```
  project/
  └── internal/
      ├── converter/          # 存放 abgen 指令及生成的代码
      │   ├── directives.go
      │   ├── generated.go
      │   └── custom.go       # 用户自定义实现
      └── models/             # 数据模型定义
  ```
- **配置管理**:
  - 建议将所有 `abgen` 指令集中到一个或少数几个文件（如 `converter/directives.go`）中，便于管理。
- **版本控制**:
  - 将 `abgen` 生成的文件 (`generated.go`, `custom.go`) 纳入版本控制，以确保构建的可复现性。
