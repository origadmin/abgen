# 01 - 复杂类型分析测试

## 测试目标
本测试用例旨在**隔离地**验证 `analyzer` 模块在处理各种复杂Go类型时的健壮性和准确性。

它的核心任务是，给定一个包含复杂类型定义的Go源文件，验证 `analyzer.PackageWalker` 能否为每一个目标类型正确地构建出相应的 `TypeInfo` 数据结构，包括其种类（Kind）、底层类型（Underlying）、字段（Fields）等所有细节。

此测试**不关心**任何代码生成的结果，仅关注类型结构分析的正确性。

## 测试场景
在一个Go源文件中定义一系列复杂的类型，涵盖Go语言的主要类型系统特性：
- 基础结构体 (`BaseStruct`)
- 包含所有复杂类型的聚合结构体 (`User`)
- 对外部包类型的别名 (`UserAlias`)
- 对指针的有名类型定义 (`DefinedPtr`) 和别名 (`AliasPtr`)
- 对指针切片的有名类型定义 (`DefinedSliceOfPtrs`) 和别名 (`AliasSliceOfPtrs`)
- 对切片指针的有名类型定义 (`DefinedPtrToSlice`) 和别名 (`AliasPtrToSlice`)
- 对切片的切片的有名类型定义 (`DefinedSliceOfSlices`) 和别名 (`AliasSliceOfSlices`)
- 对Map的有名类型定义 (`DefinedMap`) 和别名 (`AliasMap`)
- 多层指针 (`TriplePtr`)
- 对基础类型的指针 (`MyPtr`)

## 输入数据
- `source/types.go`: 包含上述所有复杂类型定义的Go源文件。
- `external/types.go`: 包含一个被主测试文件引用的外部类型 `external.User`。

## 预期输出
此阶段不生成 `.go` 文件。

预期的输出是在 `package_walker_test.go` 的单元测试中，通过 `walker.Find(fqn)` 查找到的每一个 `TypeInfo` 对象都具有正确的内部结构。例如：
- 对于 `DefinedPtr`，其 `Kind` 应为 `Pointer`，其 `Underlying` 应为一个 `Kind` 为 `Struct` 的 `TypeInfo`。
- 对于 `DefinedMap`，其 `Kind` 应为 `Map`，其 `KeyType` 和 `Underlying` (值类型) 都应被正确解析。
- 对于 `UserAlias`，其 `IsAlias` 应为 `true`，且其 `Fields` 列表应能正确反映其引用的 `external.User` 的字段。

## 验证点
- [ ] `PackageWalker` 能够一次性加载包含所有依赖（如 `external` 包）的完整包图。
- [ ] `walker.Find()` 能够成功查找到所有定义的类型。
- [ ] 基础结构体的字段被正确解析。
- [ ] 别名类型 (`type T = U`) 的 `IsAlias` 标志位为 `true`。
- [ ] 有名类型 (`type T U`) 的 `IsAlias` 标志位为 `false`。
- [ ] 指针、切片、数组、Map等复合类型的 `Kind` 和 `Underlying` 关系被正确构建。
- [ ] 多层嵌套的复杂类型（如 `[]*map[string]***MyType`）能够被正确地递归解析。
- [ ] 跨包引用的类型能够被正确解析。

## 相关文档
- `D:\workspace\project\golang\origadmin\framework\tools\abgen\FIX_PLAN.md` (阶段1.5)

## 状态记录
- **2024-08-06**: 创建此测试用例，作为 `01_type_analysis` 阶段的第一个隔离测试。
