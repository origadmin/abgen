# Map Conversions

## 测试目标
验证Map类型之间的转换，包括：
- 值类型Map的转换（map[K]V ↔ map[K]*V）
- 键类型Map的转换（map[K1]V ↔ map[K2]V）
- 组合转换（map[K1]V ↔ map[K2]*V）
- nil Map的安全处理

## 转换场景

### 1. 值到指针值转换
```go
// 源 (ent)
Metadata map[string]string  // 值到值

// 目标 (types)
Metadata map[string]*string // 值到指针

// 预期生成
func ConvertMetadataToMetadataMap(metadata map[string]string) map[string]*string {
    if metadata == nil {
        return nil
    }
    result := make(map[string]*string, len(metadata))
    for k, v := range metadata {
        result[k] = &v
    }
    return result
}
```

### 2. 指针到值转换
```go
// 源 (ent)
Tags map[int]string

// 目标 (types)
Tags map[int32]*string // 键类型转换 + 值到指针

// 预期生成
func ConvertTagsToTagsMap(tags map[int]string) map[int32]*string {
    if tags == nil {
        return nil
    }
    result := make(map[int32]*string, len(tags))
    for k, v := range tags {
        result[int32(k)] = &v
    }
    return result
}
```

### 3. 数值精度转换
```go
// 源 (ent)
Scores map[string]int

// 目标 (types) 
Scores map[string]int64

// 预期生成
func ConvertScoresToScoresMap(scores map[string]int) map[string]int64 {
    if scores == nil {
        return nil
    }
    result := make(map[string]int64, len(scores))
    for k, v := range scores {
        result[k] = int64(v)
    }
    return result
}
```

## 关键设计点

### nil处理策略
- **源Map为nil**：返回nil（保持nil语义）
- **目标Map初始化**：预分配容量提升性能
- **值转换**：应用相应的值类型转换规则

### 类型转换优先级
1. **键类型转换**：int → int32 → int64
2. **值类型转换**：值 → 指针 + 精度转换
3. **组合转换**：键和值的复合转换

### 性能优化
- **容量预分配**：基于源Map长度预分配
- **内存复用**：当可能时复用现有Map
- **批量转换**：避免逐个元素的开销

## 错误处理

### 类型不匹配
- **键类型不兼容**：编译时检查或运行时panic
- **值类型不兼容**：应用默认转换或返回错误
- **混合类型**：记录警告并尝试转换

### 边界情况
- **空Map**：返回空Map（非nil）
- **大Map**：考虑内存限制和性能
- **循环引用**：Map中包含自身引用

## 验证点
- [ ] 基础Map转换（相同键类型）
- [ ] 键类型转换（int ↔ int32）
- [ ] 值类型转换（值 ↔ 指针）
- [ ] 组合转换（键+值同时转换）
- [ ] nil安全处理
- [ ] 大容量Map的性能
- [ ] 空Map和nil Map的区别