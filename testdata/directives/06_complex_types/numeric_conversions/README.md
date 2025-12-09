# Numeric Conversions

## 测试目标
验证数值类型之间的安全转换，包括：
- 整数宽度转换（int ↔ int32 ↔ int64）
- 浮点精度转换（float32 ↔ float64）
- 整浮转换（int ↔ float32/float64）
- 溢出和精度丢失的处理

## 转换场景

### 1. 整数宽度转换
```go
// 源 (ent)
Age int  // 宽度转换

// 目标 (types)
Age int32

// 预期生成
func ConvertAgeToAgeNumeric(age int) int32 {
    // 检查溢出
    if age < math.MinInt32 || age > math.MaxInt32 {
        // 处理溢出：截断或使用默认值
        return 0
    }
    return int32(age)
}
```

### 2. 整数到浮点数转换
```go
// 源 (ent)
Score int

// 目标 (types)
Score float64

// 预期生成
func ConvertScoreToScoreNumeric(score int) float64 {
    return float64(score)
}
```

### 3. 浮点精度转换
```go
// 源 (ent)
Rating float32

// 目标 (types)
Rating float64

// 预期生成
func ConvertRatingToRatingNumeric(rating float32) float64 {
    return float64(rating)
}

// 源 (ent)
Balance float64

// 目标 (types)
Balance float32

// 预期生成
func ConvertBalanceToBalanceNumeric(balance float64) float32 {
    // 检查是否超出float32范围
    if balance > math.MaxFloat32 {
        return math.MaxFloat32
    }
    if balance < -math.MaxFloat32 {
        return -math.MaxFloat32
    }
    return float32(balance)
}
```

## 关键设计点

### 溢出处理策略
1. **截断策略**：超出范围时截断到最大/最小值
2. **包装策略**：使用类似饱和运算的处理方式
3. **错误策略**：返回错误或panic
4. **默认策略**：返回零值

### 精度丢失处理
- **float64 → float32**：可能丢失精度
- **int → float32/64**：大整数可能丢失精度
- **float32/64 → int**：丢失小数部分

### 特殊值处理
- **NaN**：浮点数的NaN处理
- **Inf**：正负无穷大的处理
- **零值**：正零和负零的处理

## 转换函数设计

### 安全转换函数
```go
func ConvertInt32ToInt64(value int32) int64 {
    return int64(value) // 总是安全
}

func ConvertInt64ToInt32(value int64) (int32, error) {
    if value < math.MinInt32 || value > math.MaxInt32 {
        return 0, fmt.Errorf("value %d overflows int32", value)
    }
    return int32(value), nil
}
```

### 饱和转换函数
```go
func ConvertInt64ToInt32Saturated(value int64) int32 {
    if value > math.MaxInt32 {
        return math.MaxInt32
    }
    if value < math.MinInt32 {
        return math.MinInt32
    }
    return int32(value)
}
```

## 验证点
- [ ] 整数宽度转换（安全检查）
- [ ] 整浮转换（精度处理）
- [ ] 浮点精度转换（溢出检查）
- [ ] 特殊值处理（NaN, Inf）
- [ ] 边界值测试（最大/最小值）
- [ ] 性能基准测试
- [ ] 错误处理策略