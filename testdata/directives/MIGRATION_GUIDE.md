# 测试用例迁移指南

## 迁移映射关系

### 已完成的映射

| 旧测试用例 | 新结构化位置 | 状态 | 说明 |
|-----------|-------------|------|------|
| `p01_basic/` | `01_basic_modes/simple_bilateral/` | ✅ 已映射 | 基础双方转换 |
| `example_trilateral/` | `01_basic_modes/standard_trilateral/` | ✅ 已映射 | 标准三方转换 |
| `p02_alias/` | `04_type_aliases/auto_generate_aliases/` | ✅ 已映射 | 类型别名自动生成 |
| `p03_remap/` | `05_field_mapping/simple_field_remap/` | ✅ 已映射 | 简单字段重映射 |
| `p04_slice/` | `06_complex_types/slice_conversions/` | ✅ 已映射 | 切片类型转换 |
| `p05_enum/` | `06_complex_types/enum_string_to_int/` | ✅ 已映射 | 枚举字符串到整型 |
| `p06_custom/` | `07_custom_rules/custom_function_rules/` | ✅ 已映射 | 自定义函数规则 |

## 迁移策略

### 阶段1：并行运行（当前阶段）
- ✅ 保留所有原有测试用例（向后兼容）
- ✅ 创建新的结构化测试用例
- 🔄 逐步验证新旧测试的一致性

### 阶段2：逐步替换
1. 验证映射测试用例的功能一致性
2. 更新CI/CD流水线使用新结构
3. 文档和示例迁移到新结构

### 阶段3：清理旧结构
1. 确认所有功能在新结构中正常工作
2. 逐步废弃旧的p系列测试
3. 完全迁移到新结构

## 新结构优势

### 🎯 更清晰的分类
- **按功能分组**：将相关功能组织在同一目录
- **层次化结构**：从简单到复杂的渐进式组织
- **命名语义化**：目录名称直接反映测试功能

### 📊 更好的覆盖度追踪
- **优先级明确**：P0/P1/P2优先级分类
- **功能完整性**：每个类别下的具体场景覆盖
- **缺失识别**：容易发现未实现的测试场景

### 🔧 更易维护
- **标准化结构**：每个测试用例遵循相同模板
- **文档化要求**：每个测试都有详细的README
- **扩展友好**：新增功能有明确的归属位置

## 测试用例标准模板

每个新的测试用例应包含：

```
test_case_name/
├── directives.go          # 测试指令（必需）
├── README.md             # 测试说明（必需）
├── custom.go             # 自定义函数（可选）
├── expected.golden       # 预期输出（可选）
├── source/               # 独立源包（可选）
└── target/               # 独立目标包（可选）
```

### directives.go 模板
```go
package directives

import (
    _ "source/package"
    _ "target/package"
)

//go:abgen:package:path=source/package,alias=source
//go:abgen:package:path=target/package,alias=target

//go:abgen:pair:packages="source,target"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="Suffix"

// 说明：测试目标和预期行为
```

### README.md 模板
```markdown
# 测试用例名称

## 测试目标
简述测试目标和功能

## 转换场景
具体的转换示例和代码片段

## 关键设计点
测试的核心设计考虑

## 验证点
需要验证的具体功能点
```

## 下一步行动

### 立即执行
1. **✅ 创建结构化目录**：完成新目录结构创建
2. **✅ 更新测试文件**：修改generator_test.go使用新结构
3. **🔄 生成Golden文件**：为新测试用例创建expected.golden文件
4. **🧪 验证测试**：运行`.\validate_new_tests.ps1`确保功能正常

### 清理阶段
5. **🧹 清理旧目录**：运行`.\cleanup_old_tests.ps1`移除p系列目录
6. **✅ 再次验证**：确保清理后所有测试仍正常
7. **📝 更新文档**：将文档和示例迁移到新结构

### 近期规划
1. **扩展新功能**：实现enum_int_to_string等缺失测试
2. **性能对比**：新旧结构的性能和可维护性对比
3. **团队培训**：确保团队理解新结构的使用方法

### 长期目标
1. **完全迁移**：废弃旧的p系列测试结构
2. **自动化工具**：开发测试用例生成工具
3. **社区推广**：将新结构推广到其他类似项目

## 清理脚本说明

### validate_new_tests.ps1
- **用途**：验证新测试结构完整性
- **功能**：检查目录结构、文件存在性、运行测试
- **使用时机**：清理前必须执行

### cleanup_old_tests.ps1
- **用途**：清理旧的p系列测试目录
- **功能**：删除p01-p06目录和对应golden文件
- **安全机制**：需要用户确认后才执行

### 清理后状态
```
testdata/directives/
├── 01_basic_modes/     # ✅ 保留
├── 04_type_aliases/    # ✅ 保留
├── 05_field_mapping/   # ✅ 保留
├── 06_complex_types/   # ✅ 保留
├── 07_custom_rules/    # ✅ 保留
├── bug_fix_001/        # ✅ 保留（特殊用例）
└── test_menu_pb_issue/ # ✅ 保留（特殊用例）

# 以下将被清理：
# p01_basic/, p02_alias/, p03_remap/, p04_slice/, p05_enum/, p06_custom/, example_trilateral/
```