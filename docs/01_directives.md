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
当 `abgen` 生成切片类型（例如 `[]*pb.User`）的别名时，其命名规则是基于元素类型的命名，并在此基础上添加复数形式。

规则：`[元素类型别名基础名]` + `s`

具体生成逻辑：
1.  首先，确定切片元素类型的别名基础名（即去除 `Prefix` 和 `Suffix` 的原始类型名）。例如，如果 `User` 被处理为 `UserPB`，则其基础名为 `User`。
2.  将此基础名复数化（通常是添加 `s`）。例如，`User` -> `Users`。
3.  将此复数化的基础名，重新应用原始元素类型别名中的 `Prefix` 和 `Suffix`。例如，`Users` + `PB` -> `UsersPB`。

**示例**:
如果 `abgen` 将 `pb.User` 别名为 `UserPB` (应用了 `target:suffix="PB"` 规则)，那么对于 `[]*pb.User` 这样的切片类型，其别名将被生成为 `UsersPB`。

在生成的代码中，将使用 `UsersPB` 这样的别名来指代切片类型，以提高可读性。

#### 字段级转换函数命名 (Field-Level Conversion Function Naming)
当两个结构体进行转换时，如果某个字段因类型不兼容而无法直接赋值或使用基本类型转换（例如，`string` ↔ `int`，或两个都叫 `Status` 但底层类型不同的具名类型），`abgen` 不会生成通用的转换函数（如 `ConvertStringToInt`）。

相反，它会生成一个**合成的、上下文明确的**函数调用。其命名规则为：
`Convert[源结构体别名][字段名]To[目标结构体别名][字段名]`

**示例**: 在 `UserBilateral` (是 `types.User` 的别名) 到 `User` (是 `ent.User` 的别名) 的转换中，`Status` 字段（`int32` ↔ `string`）的转换函数调用将被命名为 `ConvertUserBilateralStatusToUserStatus`。

这样做可以确保为每个特定的字段转换任务生成唯一且可读的函数调用。`abgen` 不会去“推测”这个合成函数的实现逻辑，而是会在 `custom.go` 文件中生成一个带 `panic` 的**桩函数 (Stub Function)**，要求用户必须为其提供具体的实现。

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

---

## 内部实现原理 (Internal Implementation)

本章节包含 `abgen` 的内部工作流和设计细节，主要面向高级用户和贡献者。

### 解析与执行流程

`abgen` 的工作流程始于对代码的深入分析，然后根据分析结果和用户定义的规则生成代码。

#### 步骤 1：本地包解析 (Local Package Parsing)
**目标**: 在处理任何 `//go:abgen:` 指令之前，`abgen` 首先扫描当前包内的所有 Go 源文件，以构建关于本地类型别名的完整双向映射表（`alias <-> FQN`）。这是后续指令解析和代码生成的基础。

#### 步骤 2：源包与目标包的按需解析
**目标**: 深入分析由指令指定的源包和目标包，为每个需要转换的类型（主要是 `struct`）创建一个包含其所有字段（名称、类型 FQN、标签等）的内部结构模型。此过程按需递归解析依赖，避免加载不相关的包。

#### 步骤 3：代码生成
**目标**: 遍历所有已确定的转换任务，根据解析好的类型模型和全局规则集，生成最终的 Go 源代码文件。
1.  **确定生成任务**: 根据 `convert` 和 `pair` 指令及 `direction` 规则，创建所有待生成函数的列表。
2.  **生成转换函数**: 为每个任务生成函数。
    - **函数命名与签名**: 根据命名规则生成函数名和 `func(from *S) *T` 签名。
    - **函数体生成**:
      - **最高优先级**: 检查是否有 `convert:rule` 指定了自定义函数。
      - **递归调用**: 检查字段类型之间是否是另一个已知的转换任务，并生成递归调用。
      - **同名/同类型映射**: 默认进行同名、同类型字段的直接赋值。
3.  **管理导入与别名**: 在生成代码的同时，智能地处理所有外部类型的引用，收集必要的 `import` 路径，并根据 `alias:generate` 规则决定是生成 `type` 别名还是直接使用 `pkg.Type`。
4.  **生成自定义函数桩 (Stubs)**: 在 `custom.gen.go` 文件中为用户在 `rule` 指令中指定的所有自定义函数提供 `panic` 占位符实现，以确保项目可立即编译。
5.  **组装与格式化**: 将所有生成的内容组合成格式正确的 Go 文件。

### 类型依赖解析问题与解决方案
**问题**: 在 `abgen` 分析代码时，有时会遇到接口定义中引用了**即将被 `abgen` 生成但尚未存在**的类型（例如 `MenuPB`），导致 Go 类型检查器报错，中断 `abgen` 进程。

**解决方案**: `abgen` 采用**虚拟类型注册 + 两阶段解析**的策略来解决。
1.  **预测**: `abgen` 首先在不进行完整类型检查的情况下解析所有指令，根据 `source/target:suffix` 等命名规则，**预测**将要生成的类型名称（如 `MenuPB`）。
2.  **注册**: 将这些预测出的类型名作为“虚拟类型”注册到解析器内部。
3.  **容错解析**: 在第二阶段进行完整类型分析时，如果遇到一个在代码中找不到但已注册为虚拟类型的名称，解析器会认为它是有效的，从而避免了“undefined type”错误。
4.  **延迟验证**: 在最终代码生成时，所有类型都已确定，完成最终验证。

此机制确保了即使在接口中预先引用了待生成的类型，`abgen` 也能顺利完成解析和代码生成。

### 版本兼容性
- **版本策略**: `abgen` 遵循语义化版本（SemVer）。在主版本内保持 API（指令语法）的向后兼容。
- **已知限制**:
  - 对 Go 泛型的支持尚不完整。
  - 复杂的类型循环引用可能需要用户通过 `convert:rule` 手动处理。