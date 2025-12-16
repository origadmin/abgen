# Abgen 命名规则与生成逻辑

本文档旨在明确 `abgen` 在生成别名（Alias）和函数名时的核心逻辑与处理流程，作为代码实现的唯一依据。

## 1. 核心原则

1.  **职责单一**: 每个函数只做一件事。例如，驼峰化函数不应关心前缀/后缀，反之亦然。
2.  **流程确定**: 命名规则的应用顺序必须是固定的、可预测的。
3.  **配置优先**: 用户的命名规则配置（`naming_rules`）具有最高优先级。
4.  **歧义处理**: 仅在必要时（且无用户自定义规则时）进行自动歧义处理。

## 2. 关键概念

*   **原始名称 (Raw Name)**: 从源代码中直接获取的类型名称，如 `User`, `Item`。
*   **基础名称 (Base Name)**: 经过初步处理的名称。对于普通类型，它就是其 `Raw Name`；对于切片类型，它是其元素 `Raw Name` 的复数形式（如 `Item` -> `Items`）。
*   **配置前缀/后缀**: 在 `.abgen.yaml` 中为 `source` 或 `target` 配置的 `prefix` 和 `suffix`。
*   **歧义后缀**: 当两个类型 `Raw Name` 相同但包路径不同，且**没有**为它们配置任何前缀/后缀时，自动添加的 `Source` 或 `Target` 后缀。
*   **最终别名 (Final Alias)**: 在生成的代码中使用的最终类型名称。

## 3. 别名生成流程 (The Process)

对于每一个需要生成别名的类型 (`TypeInfo`)，严格遵循以下步骤：

**输入**:
*   `info`: 当前类型的 `model.TypeInfo`。
*   `isSource`: 一个布尔值，`true` 表示是源类型，`false` 表示是目标类型。
*   `needsDisambiguation`: 一个布尔值，表示是否需要进行歧义处理。

**步骤**:

1.  **提取基础名称 (Get Base Name)**
    *   从 `info` 中获取 `Raw Name`。
    *   **驼峰化**: 对 `Raw Name` 应用 `ToCamelCase`，确保其符合Go的大写驼峰规范（例如，`user` -> `User`）。
    *   **复数化 (仅切片)**: 如果类型是切片，在驼峰化的基础上添加 `s`（例如，`Item` -> `Items`）。
    *   **输出**: `CamelCasedBaseName`。

2.  **应用命名规则 (Apply Naming Rules)**
    *   获取当前方向 (`isSource`) 对应的 `ConfiguredPrefix` 和 `ConfiguredSuffix`。
    *   **驼峰化前后缀**: 分别对 `ConfiguredPrefix` 和 `ConfiguredSuffix` 应用 `ToCamelCase`。
    *   **拼接**: 将它们与 `CamelCasedBaseName` 组合：`FinalAlias = ToCamelCase(ConfiguredPrefix) + CamelCasedBaseName + ToCamelCase(ConfiguredSuffix)`。

3.  **处理歧义 (Handle Disambiguation)**
    *   **检查条件**: 当且仅当 `needsDisambiguation` 为 `true` **并且** 在步骤 2 中没有应用任何 `ConfiguredPrefix` 或 `ConfiguredSuffix` 时，执行此步骤。
    *   **追加后缀**: 在 `FinalAlias` 的末尾追加 `Source` 或 `Target`。

4.  **完成**
    *   经过以上步骤得到的 `FinalAlias` 即为最终的别名。

## 4. `ToCamelCase` 函数定义

`ToCamelCase(s string)` 函数是一个纯粹的工具，其职责是：
*   接收一个字符串。
*   按非字母数字字符（如 `_`, `-`）分割。
*   将每个部分的第一个字母大写。
*   将所有部分连接起来。
*   **示例**: `to_camel_case("user_name")` -> `"UserName"`, `to_camel_case("user")` -> `"User"`, `to_camel_case("usercustom")` -> `"Usercustom"`。

## 5. 示例

### 示例 A: `standard_trilateral` 测试

*   **源类型**: `Raw Name` = `Resource`, `isSource` = `true`, `needsDisambiguation` = `true`
    *   **Config**: `source_suffix: "Source"`
    *   **流程**:
        1.  Base Name: `ToCamelCase("Resource")` -> `"Resource"`
        2.  Apply Rules: `ConfiguredSuffix` 是 `"Source"`。`FinalAlias` = `"Resource"` + `ToCamelCase("Source")` -> `"ResourceSource"`
        3.  Disambiguation: 因为应用了 `ConfiguredSuffix`，跳过此步。
    *   **结果**: `ResourceSource` (正确)

*   **目标类型**: `Raw Name` = `Resource`, `isSource` = `false`, `needsDisambiguation` = `true`
    *   **Config**: `target_suffix: "Trilateral"`
    *   **流程**:
        1.  Base Name: `ToCamelCase("Resource")` -> `"Resource"`
        2.  Apply Rules: `ConfiguredSuffix` 是 `"Trilateral"`。`FinalAlias` = `"Resource"` + `ToCamelCase("Trilateral")` -> `"ResourceTrilateral"`
        3.  Disambiguation: 因为应用了 `ConfiguredSuffix`，跳过此步。
    *   **结果**: `ResourceTrilateral` (正确)

### 示例 B: `package_level_conversion` 测试

*   **源类型**: `Raw Name` = `User`, `isSource` = `true`, `needsDisambiguation` = `true`
    *   **Config**: (无相关前后缀配置)
    *   **流程**:
        1.  Base Name: `ToCamelCase("User")` -> `"User"`
        2.  Apply Rules: 无配置，`FinalAlias` = `"User"`
        3.  Disambiguation: `needsDisambiguation` 为 `true` 且无配置规则，追加 `Source`。`FinalAlias` -> `"UserSource"`
    *   **结果**: `UserSource` (正确)

*   **目标类型**: `Raw Name` = `User`, `isSource` = `false`, `needsDisambiguation` = `true`
    *   **Config**: (无相关前后缀配置)
    *   **流程**:
        1.  Base Name: `ToCamelCase("User")` -> `"User"`
        2.  Apply Rules: 无配置，`FinalAlias` = `"User"`
        3.  Disambiguation: `needsDisambiguation` 为 `true` 且无配置规则，追加 `Target`。`FinalAlias` -> `"UserTarget"`
    *   **结果**: `UserTarget` (正确)

### 示例 C: `usercustom` 问题

*   **目标类型**: `Raw Name` = `usercustom`, `isSource` = `false`
    *   **Config**: `target_suffix: "custom"`
    *   **流程**:
        1.  Base Name: `ToCamelCase("usercustom")` -> `"Usercustom"`
        2.  Apply Rules: `ConfiguredSuffix` 是 `"custom"`。`FinalAlias` = `"Usercustom"` + `ToCamelCase("custom")` -> `"UsercustomCustom"` (错误!)

**流程修正**:

步骤 2 **必须** 能够识别并拆分 `Raw Name`。修正后的流程：

**2. 应用命名规则 (修正)**
    a. 获取 `ConfiguredPrefix` 和 `ConfiguredSuffix`。
    b. **拆分**: 检查 `Raw Name` 是否以 `ConfiguredSuffix` 结尾（不区分大小写）。
        *   是: `BasePart` = `Raw Name` 去掉后缀的部分, `SuffixPart` = `ConfiguredSuffix`。
        *   否: `BasePart` = `Raw Name`, `SuffixPart` = `""`。
    c. **驼峰化**: `CamelBase` = `ToCamelCase(BasePart)`, `CamelSuffix` = `ToCamelCase(SuffixPart)`, `CamelPrefix` = `ToCamelCase(ConfiguredPrefix)`。
    d. **拼接**: `FinalAlias = CamelPrefix + CamelBase + CamelSuffix`。

**修正后的 `usercustom` 示例**:
*   **流程**:
    1.  Base Name: `Raw Name` 是 `"usercustom"`。
    2.  Apply Rules (修正后):
        a. `ConfiguredSuffix` 是 `"custom"`。
        b. `rawTypeName` ("usercustom") 以 `"custom"` 结尾。拆分: `BasePart` = `"user"`, `SuffixPart` = `"custom"`。
        c. 驼峰化: `CamelBase` = `"User"`, `CamelSuffix` = `"Custom"`, `CamelPrefix` = `""`。
        d. 拼接: `FinalAlias` = `""` + `"User"` + `"Custom"` -> `"UserCustom"`。
    3.  Disambiguation: 因为应用了 `ConfiguredSuffix`，跳过此步。
*   **结果**: `UserCustom` (正确)
