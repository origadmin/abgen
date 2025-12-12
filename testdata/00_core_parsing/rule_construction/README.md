# 01 - 全规则构建测试

## 测试目标
本测试用例旨在全面验证 `config` 包的指令解析功能在理想情况下的正确性。它将模拟一组复杂的、具有代表性的 `go:abgen` 指令，并验证解析器能否根据这些指令构建一个结构正确、内容完整的 `Config` 对象。

这是 `00_core_parsing` 阶段的核心验收测试。

## 测试场景
在一个典型的项目中，开发者需要定义多种转换规则：
1.  为外部包 `ent` 和 `pb` 定义别名。
2.  定义一个从 `ent` 包到 `pb` 包的配对关系。
3.  定义一个从 `github.com/another/pkg` (一个没有别名的完整路径包) 到 `pb` 包的配d对关系。
4.  定义一个全局的命名规则（为目标类型添加 `Model` 后缀）。
5.  定义一个核心的 `ent.User` 到 `pb.User` 的转换规则，此规则包含：
    - 双向转换 (`direction=both`)。
    - 忽略特定字段 (`ignore=PasswordHash;Salt`)。
    - 重映射特定字段 (`remap=CreatedAt:CreatedTimestamp`)。
6.  定义另一个简单的、仅包含源和目标的转换规则。

## 输入数据
输入数据是位于 `source.go` 文件中的一组 `//go:abgen` 指令。

**指令详情:**
```go
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
```

## 预期输出
此阶段不生成 `.go` 文件。预期的输出是一个在内存中被正确构建的 `config.Config` 对象。其关键字段应包含：

- **`PackageAliases`**: `{"ent": "github.com/my/project/ent/v2", "pb": "github.com/my/project/pb"}` (验证覆盖逻辑)
- **`NamingRules.TargetSuffix`**: `"Model"` (验证全局规则)
- **`PackagePairs`**: 包含两个 `PackagePair` 对象，正确反映 `(ent, pb)` 和 `(another/pkg, pb)` 的配对。
- **`ConversionRules`**: 包含两个 `ConversionRule` 对象，且第一个规则的 `Direction`, `FieldRules.Ignore`, `FieldRules.Remap` 字段被正确填充。

## 验证点
- [ ] `config.Parser` 能够无错误地解析所有指令。
- [ ] `PackageAliases` 正确应用了覆盖规则，`ent` 指向 `v2` 路径。
- [ ] `NamingRules` 被正确设置为全局规则。
- [ ] `PackagePairs` 正确累积了所有配对关系，并能同时处理别名和完整路径。
- [ ] `ConversionRules` 正确累积了所有转换规则。
- [ ] 复杂的 `convert` 规则中的 `direction`, `ignore`, `remap` 参数被正确解析并填充到对应的 `ConversionRule` 对象中。

## 相关文档
- `D:\workspace\project\golang\origadmin\framework\tools\abgen\FIX_PLAN.md`
- `D:\workspace\project\golang\origadmin\framework\tools\abgen\TESTDATA_RESTRUCTURE_PLAN.md`

## 状态记录
- **2024-08-06**: 创建此测试用例，作为 `00_core_parsing` 阶段的验收标准。
