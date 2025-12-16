# abgen 架构与设计文档

## 1. 核心领域概念 (Ubiquitous Language)

- **`TypeInfo`**: `abgen` 内部表示一个 Go 类型的核心结构，包含其原始名称 (`Name`)、导入路径 (`ImportPath`)、种类 (`Kind`)、底层类型 (`Underlying`) 及字段 (`Fields`) 等元数据。
- **`ConversionRule`**: 定义了源类型 (Source) 和目标类型 (Target) 之间的转换关系，包括转换方向 (`Direction`) 和字段级别的特殊规则 (如 `remap`, `ignore`)。
- **`Parser`**: **解析器**。其唯一职责是读取 Go 源代码和 `//go:abgen` 指令，构建出原始的、未经加工的 `TypeInfo` 和 `ConversionRule` 集合。
- **`Generator`**: **总编排器**。是代码生成过程的引擎和大脑，负责协调 `Parser` 和 `Namer`，并严格按照预定流程执行所有步骤。
- **`Namer`**: **命名器**。一个**无状态**的工具，负责所有与命名相关的计算，如生成类型别名 (`FinalAlias`) 和函数名。它本身不存储任何配置或状态，所有决策所需信息均由 `Generator` 在调用时提供。

## 2. 核心设计原则

1.  **职责分离 (Separation of Concerns)**:
    - `Parser` 只负责“读懂”用户的意图和代码结构。
    - `Generator` 只负责“决策”和“协调”，即决定在何时、以何种顺序、用哪些信息去调用其他模块。
    - `Namer` 只负责“命名”，它是一个纯粹的计算单元。

2.  **`Generator` 作为总编排器**: 所有逻辑流转、状态管理和模块交互均由 `Generator` 统一控制。这种集中式控制解决了过去因逻辑分散导致的各种状态不一致问题。

3.  **无状态 `Namer`**: `Namer` 的所有方法都是幂等的。对于相同的输入，它总是返回相同的结果。它不再自己解析或存储配置，而是依赖 `Generator` 在调用时传入完整的上下文信息（如 `isSource`、前缀/后缀等），从而保证了行为的可预测性。

## 3. 代码生成流程 (The Generation Flow)

`abgen` 的执行流程被严格划分为清晰的阶段，全部由 `Generator` 驱动。

### 阶段 1: 解析 (`Parser` 职责)

1.  `Generator` 调用 `Parser` 来分析目标目录中的所有 Go 文件。
2.  `Parser` 读取所有 `//go:abgen` 指令，并构建初始的 `ConversionRule` 列表。
3.  `Parser` 使用 Go 的类型检查工具 (`go/types`) 解析代码，为每个涉及的类型创建一个原始的 `TypeInfo` 对象，并存入 `g.typeInfos` 中。
4.  **关键**: 在此阶段，`Parser` 不做任何命名决策或别名生成。`TypeInfo` 中存储的是最原始的类型信息。

### 阶段 2: 编排与生成 (`Generator` 职责)

这是 `abgen` 的核心，`Generator` 在此阶段执行一系列严格有序的操作。

**步骤 2.1: 发现隐式规则 (Discover Implicit Rules)**
- `Generator` 遍历初始规则列表，处理 `pair:packages` 这样的批量指令，将其扩展为一系列具体的 `Type-To-Type` 的 `ConversionRule`。

**步骤 2.2: 填充 `Namer` 上下文 (Populate Namer Context)**
- **(关键步骤)** `Generator` 遍历所有 `ConversionRule`，识别出所有被用作“源”的包路径。
- 它将这些路径集合传递给 `namer.PopulateSourcePkgs()`。
- 这一步为 `Namer` 提供了全局视角，使其在后续步骤中能够准确判断任意一个 `TypeInfo` 究竟是扮演“源”还是“目标”的角色。

**步骤 2.3: 智能歧义处理 (Intelligent Disambiguation)**
- `Generator` 再次遍历顶层的 `ConversionRule`。
- 对于每一对转换（`Source` ↔ `Target`），它检查：
  1.  源类型和目标类型的**原始名称是否相同** (`sourceInfo.Name == targetInfo.Name`)。
  2.  用户是否**没有**为这对转换提供任何自定义的 `prefix/suffix` 命名规则。
- 只有当**两个条件同时满足**时，`Generator` 才会决定需要为这次转换应用默认的 `Source`/`Target` 后缀以消除歧义。

**步骤 2.4: 别名预计算与存储 (Pre-calculate & Store Aliases)**
- `Generator` 为每一个转换规则中的顶层 `Source` 和 `Target` 类型计算它们的最终别名 (`FinalAlias`)。
- 在调用 `namer.GetAlias()` 时，它会传入从用户配置或上一步“智能歧义处理”中确定的前后缀。
- 计算出的别名被存储在 `g.aliasMap` 中，这是一个 `FQN -> FinalAlias` 的映射，作为后续所有命名查找的缓存和唯一真实来源。
- **注意**: 此阶段**不处理**结构体内部字段等嵌套类型的别名。

**步骤 2.5: 生成代码 (Generate Code)**
- **写别名**: `Generator` 遍历 `g.aliasMap`，为所有在预计算阶段生成的顶层别名输出 `type FinalAlias = original.Type` 声明。
- **写转换函数**:
  - `Generator` 遍历 `ConversionRule` 列表。
  - **函数名**: 通过调用 `namer.GetFunctionName()` 生成，该函数内部会从 `g.aliasMap` 查询别名来构造函数名。
  - **函数体**: `Generator` 遍历结构体的每个字段进行转换。
    - 当需要获取一个字段的类型字符串表示时，它调用 `g.getTypeString()`。
    - **`getTypeString()` 的动态别名生成**:
      - 如果 `getTypeString()` 的入参是一个**之前未见过**的嵌套类型（例如 `*pkg.NestedStruct`），它会**在此时**为其计算并注册一个新的别名到 `g.aliasMap` 和 `g.requiredAliases` 中。
      - 这保证了即使是深层嵌套的类型，也能在需要时被正确地、一致地命名和声明。

## 4. `Namer` 模块详解

`Namer` 的核心是其无状态的命名函数。

### 4.1. `namer.GetAlias(...)`

- **职责**: 计算一个类型的 `FinalAlias`。
- **逻辑**:
  1.  获取类型的最底层基础名称 (`RawTypeName`)，例如 `[]*pkg.User` 的 `RawTypeName` 是 `User`。
  2.  对基础名称进行驼峰化，例如 `user_profile` -> `UserProfile`。
  3.  将 `Generator` 传入的、已经过处理的前后缀（驼峰化后）与驼峰化的基础名称拼接起来：`FinalAlias = Prefix + BaseName + Suffix`。
  4.  如果原始类型是切片，则在 `FinalAlias` 末尾添加 `s`。

### 4.2. 字段级不兼容转换

- 当字段类型不匹配时（如 `int32` ↔ `string`），`Generator` 不再生成上下文相关的函数名。
- 它会请求 `Namer` 提供一个基于**类型**的函数名，如 `ConvertInt32ToString`。
- `Generator` 会在 `custom.go` 文件中生成这个函数的桩，让用户实现。这个实现将在所有需要 `int32` 到 `string` 转换的地方被复用。

## 5. DDD 场景示例

我们重构后的架构能更好地支持 DDD。

### 场景: 实体 (Entity) 与数据传输对象 (DTO) 的转换

- **实体**: `domain/entity/user.go` -> `type User struct { ... }`
- **DTO**: `application/dto/user.go` -> `type User struct { ... }`

**配置**:
```go
//go:abgen:pair:packages="github.com/my/app/domain/entity,github.com/my/app/application/dto"
```

**执行流程**:
1.  `Parser` 发现 `entity.User` 和 `dto.User`。
2.  `Generator` 在“发现隐式规则”阶段，为 `entity.User` ↔ `dto.User` 创建 `ConversionRule`。
3.  `Generator` 在“智能歧义处理”阶段，发现两个 `User` 类型**同名但包不同**，且用户未提供自定义命名规则。它决定需要应用默认后缀。
4.  `Generator` 在“别名预计算”阶段，调用 `namer.GetAlias` 时：
    - 为 `entity.User` 传入 `Source` 后缀，得到别名 `UserSource`。
    - 为 `dto.User` 传入 `Target` 后缀，得到别名 `UserTarget`。
5.  `Generator` 在“代码生成”阶段，生成函数 `func ConvertUserSourceToUserTarget(...)`。

这个流程确保了即使在不同层使用相同的类型名称，也能生成清晰、无冲突的转换代码，完美契合了 DDD 中分层架构的最佳实践。
