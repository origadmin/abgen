# Pointer Conversions

## 测试目标
验证指针与值类型之间的安全转换，包括：
- 值到指针的转换（Type → *Type）
- 指针到值的转换（*Type → Type）
- nil指针的安全处理
- 多级指针的转换

## 转换场景

### 1. 值到指针转换
```go
// 源 (ent)
ID int       // 普通值类型

// 目标 (types)
Id *int      // 指针类型

// 预期生成
func ConvertIDToIDPtr(id int) *int {
    return &id
}
```

### 2. 指针到值转换
```go
// 源 (ent)  
Age *int     // 指针类型

// 目标 (types)
Age int      // 值类型

// 预期生成
func ConvertAgePtrToAge(age *int) int {
    if age == nil {
        return 0  // 零值处理
    }
    return *age
}
```

### 3. 字符串指针转换
```go
// 源
Username string

// 目标
Username *string

// 预期生成
func ConvertUsernameToUsernamePtr(username string) *string {
    return &username
}

func ConvertUsernamePtrToUsername(username *string) string {
    if username == nil {
        return ""  // 空字符串
    }
    return *username
}
```

## 关键设计点

### nil安全处理
- **指针到值**：检查nil，返回零值
- **值到指针**：总是创建新指针（不可能为nil）
- **嵌套指针**：递归处理多级指针

### 零值策略
- **数值类型**：返回0
- **字符串类型**：返回""
- **布尔类型**：返回false
- **结构体类型**：返回空结构体

### 性能考虑
- 避免不必要的指针解引用
- 复用已分配的内存（当可能时）
- 批量转换优化

## 验证点
- [ ] 值到指针的正确转换
- [ ] 指针到值的安全转换（nil处理）
- [ ] 多级指针的递归处理
- [ ] 不同数据类型的零值处理
- [ ] 转换函数的命名规范