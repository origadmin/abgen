# Abgen 命名与类型处理设计文档 (DDD 风格)

## 1. 核心领域概念 (Ubiquitous Language)

*   **TypeInfo**: `abgen` 内部表示一个 Go 类型的结构体，包含 `Name` (原始名称), `ImportPath`, `Kind`, `Underlying` (底层类型), `Fields` 等信息。
*   **RawTypeName**: 从 `TypeInfo.Name` 或 `TypeInfo.Underlying.Name` 直接获取的原始类型字符串，未经任何大小写转换或前后缀处理。例如 `user`, `resourcecustom`。
*   **CamelCasedName**: 经过 `toCamelCase` 函数处理后的字符串，符合 Go 语言的大驼峰命名规范。例如 `user_name` -> `UserName`, `usercustom` -> `Usercustom`。
*   **ConfiguredPrefix/Suffix**: 在 `.abgen.yaml` 的 `naming_rules` 中为 `source` 或 `target` 配置的 `prefix` 和 `suffix`。
*   **FinalAlias**: 在生成的 `*.gen.go` 文件中，用于 `type FinalAlias = OriginalType` 声明的类型别名。这是 `abgen` 内部对某个 `TypeInfo` 的最终命名表示。
*   **OriginalTypeString**: `FinalAlias` 声明右侧的原始类型字符串，可能包含包限定符（例如 `ent.User`）。
*   **ConversionRule**: 定义了源类型 (SourceType FQN) 和目标类型 (TargetType FQN) 之间的转换关系，以及转换方向 (Direction) 和字段规则 (FieldRules)。

## 2. 模块与职责 (Bounded Contexts)

### 2.1. `Parser` 模块 (职责: 收集原始信息)

*   **输入**: Go 源代码文件，`.abgen.yaml` 配置。
*   **输出**:
    *   `map[string]*model.TypeInfo` (`g.typeInfos`): 包含所有解析到的 `TypeInfo` 对象，键为 FQN。`TypeInfo.Name` 存储的是原始的、未经修改的类型名称。
    *   `[]*config.ConversionRule` (`g.config.ConversionRules`): 包含所有显式和隐式发现的转换规则。
*   **关键点**: `Parser` 仅负责收集原始数据，不进行任何命名转换或别名生成。

### 2.2. `Namer` 模块 (职责: 统一命名生成)

*   **输入**: `TypeInfo` 对象，`isSource` 标志，`configuredPrefix`，`configuredSuffix`。
*   **输出**: `CamelCasedName` 或 `FinalAlias`。
*   **关键方法**:
    *   `toCamelCase(s string)`: 纯粹的字符串驼峰化工具。
    *   `GetAlias(info *model.TypeInfo, isSource bool, configuredPrefix string, configuredSuffix string)`: 生成 `FinalAlias` 的核心逻辑。
    *   `getAliasedOrBaseName(info *model.TypeInfo)`: 获取一个 `TypeInfo` 的最终可用名称（可能是 `FinalAlias`，也可能是其 `CamelCasedName`）。

### 2.3. `Generator` 模块 (职责: 协调与代码输出)

*   **输入**: `g.typeInfos`, `g.config.ConversionRules`, `g.namer` 实例。
*   **输出**: 生成的 Go 代码字节流。
*   **关键字段**:
    *   `g.aliasMap map[string]string`: 存储 `TypeInfo.UniqueKey()` 到 `FinalAlias` 的映射。包含所有已计算的别名（包括顶层和按需生成的）。
    *   `g.requiredAliases map[string]struct{}`: 存储所有**需要生成 `type Alias = OriginalType` 声明**的 `TypeInfo.UniqueKey()`。
    *   `g.requiredConversionFunctions map[string]bool`: 存储所有**需要生成转换函数**的函数名。
*   **关键方法**:
    *   `populateAliases()`: 协调 `Namer` 生成所有必要的**顶层** `FinalAlias` 并填充 `g.aliasMap` 和 `g.requiredAliases`。
    *   `writeAliases()`: 根据 `g.requiredAliases` 输出 `type FinalAlias = OriginalTypeString` 声明。
    *   `writeConversionFunctions()`: 输出转换函数，函数名和参数类型都通过 `Namer` 获取。
    *   `getTypeString(info *model.TypeInfo, isSourceContext bool, ignoreAliasMap bool)`: 获取一个 `TypeInfo` 在生成代码中应使用的字符串表示（优先 `FinalAlias`，其次动态构建），并**按需收集**别名到 `g.aliasMap` 和 `g.requiredAliases`。

## 3. 核心流程: 别名生成与类型处理

### 3.1. 阶段 1: 原始信息收集 (Parser 职责)

*   **目标**: 填充 `g.typeInfos` 和 `g.config.ConversionRules`。
*   **`TypeInfo.Name` 的内容**: 始终是原始的、未经大小写转换的类型名称。例如，如果 Go 源码中定义 `type user struct {}`，则 `TypeInfo.Name` 为 `"user"`；如果定义 `type ResourceCustom struct {}`，则 `TypeInfo.Name` 为 `"ResourceCustom"`。

### 3.2. 阶段 2: 别名预处理/填充 (`Generator.populateAliases` 职责)

*   **目标**: 仅为**转换规则中直接指定的源类型和目标类型**计算并存储 `FinalAlias` 到 `g.aliasMap`。同时，将这些顶层类型的 FQN 添加到 `g.requiredAliases` 中。
*   **执行时机**: 在任何代码生成（`writeAliases`, `writeConversionFunctions`）之前。
*   **`Generator.populateAliases()` 步骤**:
    1.  遍历 `g.config.ConversionRules` 中的每一条 `rule` (SourceType FQN, TargetType FQN)。
    2.  获取 `sourceInfo = g.typeInfos[rule.SourceType]` 和 `targetInfo = g.typeInfos[rule.TargetType]`。
    3.  **判断是否需要默认 `Source/Target` 后缀**:
        *   `useDefaultDisambiguationSuffix = (sourceInfo.Name == targetInfo.Name && sourceInfo.ImportPath != targetInfo.ImportPath)`
        *   这个标志仅在 `sourceInfo` 和 `targetInfo` 之间存在同名冲突时为 `true`。
    4.  **为源类型生成别名**:
        *   `sourcePrefix, sourceSuffix := g.namer.getEffectivePrefixAndSuffix(true, useDefaultDisambiguationSuffix)`
        *   `sourceAlias := g.namer.GetAlias(sourceInfo, true, sourcePrefix, sourceSuffix)`
        *   将 `sourceAlias` 存储到 `g.aliasMap[sourceInfo.UniqueKey()]`。
        *   **将 `sourceInfo.UniqueKey()` 添加到 `g.requiredAliases` 中。**
    5.  **为目标类型生成别名**:
        *   `targetPrefix, targetSuffix := g.namer.getEffectivePrefixAndSuffix(false, useDefaultDisambiguationSuffix)`
        *   `targetAlias := g.namer.GetAlias(targetInfo, false, targetPrefix, targetSuffix)`
        *   将 `targetAlias` 存储到 `g.aliasMap[targetInfo.UniqueKey()]`。
        *   **将 `targetInfo.UniqueKey()` 添加到 `g.requiredAliases` 中。**
    6.  **不再递归处理嵌套类型**。`populateAliases` 阶段不再负责递归发现和存储所有嵌套类型的别名。

### 3.3. 阶段 3: 代码生成 (`Generator` 职责)

*   **目标**: 输出 Go 代码。

*   **`Generator.writeAliases()` 步骤**:
    1.  遍历 `g.aliasMap`。
    2.  **仅当 `fqn` 存在于 `g.requiredAliases` 中时，才生成 `type FinalAlias = OriginalTypeString` 声明。**
    3.  `OriginalTypeString` 的获取通过 `g.getTypeString(g.typeInfos[fqn], false, true)` 完成。

*   **`Generator.writeConversionFunctions()` 步骤**:
    1.  遍历 `g.config.ConversionRules` 中的每一条 `rule`。
    2.  获取 `sourceInfo = g.typeInfos[rule.SourceType]` 和 `targetInfo = g.typeInfos[rule.TargetType]`。
    3.  **生成函数名**: `funcName := g.namer.GetFunctionName(sourceInfo, targetInfo)`。
        *   `g.namer.GetFunctionName` 内部会调用 `g.namer.getAliasedOrBaseName(TypeInfo)`。
    4.  **生成参数/返回值类型字符串**: `sourceTypeStr := g.getTypeString(sourceInfo, true, false)`, `targetTypeStr := g.getTypeString(targetInfo, false, false)`。
    5.  **仅当 `funcName` 存在于 `g.requiredConversionFunctions` 中时，才生成该转换函数。**
    6.  在函数体内部，所有类型引用（包括字段类型、切片元素类型等）都通过 `g.getTypeString(TypeInfo, isSourceContext, false)` 获取其字符串表示。

## 4. `Namer` 模块的详细设计

### 4.1. `Namer.toCamelCase(s string)`

*   **职责**: 将任意字符串转换为 Go 风格的大驼峰命名。
*   **逻辑**:
    1.  替换所有非字母数字字符为空格。
    2.  按空格分割字符串为单词列表。
    3.  将每个单词的首字母大写。
    4.  连接所有单词。
*   **示例**:
    *   `toCamelCase("user_name")` -> `"UserName"`
    *   `toCamelCase("user-custom")` -> `"UserCustom"`
    *   `toCamelCase("usercustom")` -> `"Usercustom"` (不会智能拆分 `user` 和 `custom`)
    *   `toCamelCase("custom")` -> `"Custom"`

### 4.2. `Namer.getRawTypeName(info *model.TypeInfo)`

*   **职责**: 从 `TypeInfo` 中提取最底层的命名类型名称。
*   **逻辑**:
    1.  如果 `info` 为空，返回 `""`。
    2.  如果 `info` 是一个命名类型 (`info.IsNamedType()`)，返回 `info.Name`。
    3.  如果 `info` 是一个复合类型（如 `Slice`, `Pointer`, `Array`, `Map`），则递归调用 `getRawTypeName` 获取其 `Underlying` 或 `KeyType` 的名称。
    4.  否则，返回 `info.Name` (这通常是原始类型，如 `int`, `string`，或匿名结构体，但 `info.Name` 此时可能为空)。
*   **示例**:
    *   `getRawTypeName(TypeInfo{Name: "User"})` -> `"User"`
    *   `getRawTypeName(TypeInfo{Kind: Slice, Underlying: TypeInfo{Name: "Item"}})` -> `"Item"`
    *   `getRawTypeName(TypeInfo{Kind: Pointer, Underlying: TypeInfo{Name: "User"}})` -> `"User"`
    *   `getRawTypeName(TypeInfo{Kind: Slice, Underlying: TypeInfo{Kind: Pointer, Underlying: TypeInfo{Name: "User"}}})` -> `"User"`

### 4.3. `Namer.GetAlias(info *model.TypeInfo, isSource bool, configuredPrefix string, configuredSuffix string)`

*   **职责**: 根据 `TypeInfo`、方向 (`isSource`)，以及**已处理好默认歧义逻辑的** `configuredPrefix` 和 `configuredSuffix`，计算并返回 `FinalAlias`。
*   **逻辑**:
    1.  **处理原始类型**: 如果 `info.Kind == model.Primitive`，直接返回 `n.toCamelCase(info.Name)`。
    2.  **获取最底层命名类型名称**: `rawBaseName := n.getRawTypeName(info)`。
    3.  **驼峰化 `rawBaseName`**: `camelCasedBase := n.toCamelCase(rawBaseName)`。

    4.  **驼峰化 `configuredPrefix` 和 `configuredSuffix`**:
        *   `camelCasedConfiguredPrefix := n.toCamelCase(configuredPrefix)`
        *   `camelCasedConfiguredSuffix := n.toCamelCase(configuredSuffix)`

    5.  **构建基础别名**: `finalAlias = camelCasedConfiguredPrefix + camelCasedBase + camelCasedConfiguredSuffix`。

    6.  **处理切片复数化**: 如果 `info.Kind == model.Slice`，`finalAlias += "s"`。

    7.  返回 `finalAlias`。
*   **示例**:
    *   `info.Name="usercustom"`, `isSource=false`, `configuredPrefix=""`, `configuredSuffix="custom"`
        *   `rawBaseName="usercustom"`
        *   `camelCasedBase="Usercustom"`
        *   `camelCasedConfiguredSuffix="Custom"`
        *   `finalAlias="UsercustomCustom"` (正确)
    *   `info.Name="User"`, `isSource=true`, `configuredPrefix=""`, `configuredSuffix="Source"` (由 `Generator` 层面判断冲突后传入)
        *   `rawBaseName="User"`
        *   `camelCasedBase="User"`
        *   `camelCasedConfiguredSuffix="Source"`
        *   `finalAlias="UserSource"` (正确)

### 4.4. `Namer.getAliasedOrBaseName(info *model.TypeInfo)`

*   **职责**: 获取一个 `TypeInfo` 在生成代码中应使用的名称字符串。
*   **逻辑**:
    1.  如果 `info` 为空，返回 `""`。
    2.  尝试从 `g.aliasMap[info.UniqueKey()]` 中查找别名。如果找到，返回该别名。
    3.  **回退**: 如果 `g.aliasMap` 中没有找到别名，则直接返回 `n.toCamelCase(info.Name)`。这适用于那些不需要特殊别名的中间类型或未被 `populateAliases` 显式处理的类型。

## 5. `Generator.getTypeString(info *model.TypeInfo, isSourceContext bool, ignoreAliasMap bool)`

*   **职责**: 获取一个 `TypeInfo` 在生成的 Go 代码中作为类型声明（例如函数参数、返回值、`type Alias = OriginalType` 右侧）的字符串表示，并**按需收集**别名到 `g.aliasMap` 和 `g.requiredAliases`。
*   **逻辑**:
    1.  如果 `info` 为空，返回 `"interface{}"`。
    2.  **优先使用 `g.aliasMap` 中的已计算别名 (如果 `ignoreAliasMap` 为 `false`)**:
        *   `key := info.UniqueKey()`
        *   如果 `!ignoreAliasMap` 且 `g.aliasMap` 中存在 `key` 对应的别名，则返回该别名。
    3.  **如果 `g.aliasMap` 中没有 (或被忽略)，则动态构建字符串，并按需收集别名**:
        *   **对于命名类型 (`info.IsNamedType()`) 或复合类型 (Slice, Map, Array, Pointer)**：
            *   **获取上下文中的前缀/后缀**: `currentPrefix, currentSuffix := g.namer.getEffectivePrefixAndSuffix(isSourceContext, false)` (这里 `needsDisambiguation` 传递 `false`，因为 `getTypeString` 内部不负责判断顶层冲突，只使用 `populateAliases` 传递的上下文)。
            *   **按需生成别名**: 调用 `generatedAlias := g.namer.GetAlias(info, isSourceContext, currentPrefix, currentSuffix)`。
            *   **存储别名**: `g.aliasMap[key] = generatedAlias`。
            *   **标记为需要声明**: `g.requiredAliases[key] = struct{}`。
            *   **返回生成的别名**: `return generatedAlias`。
        *   **对于原始类型 (`model.Primitive`)**：直接返回 `info.Name`。
        *   **对于其他未处理的类型**：返回 `"interface{}"`。
    4.  **递归处理底层类型**: 在构建复合类型（Slice, Map, Array, Pointer）的字符串时，递归调用 `g.getTypeString` 来获取其底层类型或元素类型的正确字符串表示，并**传递 `isSourceContext` 和 `ignoreAliasMap` 参数**。

## 6. 隐式切片转换规则的发现 (`Generator.discoverImplicitConversionRules`)

*   **职责**: 根据 `pair:packages` 自动生成 `A -> B` 规则后，进一步生成 `[]A -> []B` 规则。
*   **修正逻辑**:
    1.  在生成 `sliceRule` (`SourceType: sourceSliceType.UniqueKey(), TargetType: targetSliceType.UniqueKey()`) 之前，**必须检查 `g.config.ConversionRules` 中是否已经存在与 `sourceSliceType` 或 `targetSliceType` 相关的显式规则**。
    2.  如果存在，则**不**生成该隐式切片规则，以避免重复或冲突。

## 7. 总结

这份设计文档旨在提供一个清晰、无歧义的命名和类型处理流程。通过严格遵循这些职责划分和步骤，我们期望能够解决当前存在的所有命名问题，并为未来的功能扩展提供稳定的基础。<ctrl46>}