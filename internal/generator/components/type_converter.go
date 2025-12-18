package components

import (
	"github.com/origadmin/abgen/internal/model"
)

// ConcreteTypeConverter 实现 TypeConverter 接口
type ConcreteTypeConverter struct {
	// 可根据需要添加缓存等
}

// NewTypeConverter 创建新的类型转换器
func NewTypeConverter() model.TypeConverter {
	return &ConcreteTypeConverter{}
}

// resolveConcreteType 遍历 TypeInfo 的 'Underlying' 链
// 直到找到非 Named 类型，该类型代表具体的物理类型
func (c *ConcreteTypeConverter) resolveConcreteType(info *model.TypeInfo) *model.TypeInfo {
	for info != nil && info.Kind == model.Named {
		info = info.Underlying
	}
	return info
}

// IsPointer 检查给定的 TypeInfo 是否表示指针类型
func (c *ConcreteTypeConverter) IsPointer(info *model.TypeInfo) bool {
	info = c.resolveConcreteType(info)
	return info != nil && info.Kind == model.Pointer
}

// GetElementType 返回指针、切片和数组的元素类型
func (c *ConcreteTypeConverter) GetElementType(info *model.TypeInfo) *model.TypeInfo {
	info = c.resolveConcreteType(info)
	if info == nil {
		return nil
	}

	switch info.Kind {
	case model.Pointer, model.Slice, model.Array:
		return info.Underlying
	default:
		return nil
	}
}

// GetKeyType 返回映射的键类型
func (c *ConcreteTypeConverter) GetKeyType(info *model.TypeInfo) *model.TypeInfo {
	info = c.resolveConcreteType(info)
	if info != nil && info.Kind == model.Map {
		return info.KeyType
	}
	return nil
}

// IsStruct 检查给定的 TypeInfo 是否表示结构体类型
func (c *ConcreteTypeConverter) IsStruct(info *model.TypeInfo) bool {
	info = c.resolveConcreteType(info)
	return info != nil && info.Kind == model.Struct
}

// IsSlice 检查给定的 TypeInfo 是否表示切片类型
func (c *ConcreteTypeConverter) IsSlice(info *model.TypeInfo) bool {
	info = c.resolveConcreteType(info)
	return info != nil && info.Kind == model.Slice
}

// IsArray 检查给定的 TypeInfo 是否表示数组类型
func (c *ConcreteTypeConverter) IsArray(info *model.TypeInfo) bool {
	info = c.resolveConcreteType(info)
	return info != nil && info.Kind == model.Array
}

// IsMap 检查给定的 TypeInfo 是否表示映射类型
func (c *ConcreteTypeConverter) IsMap(info *model.TypeInfo) bool {
	info = c.resolveConcreteType(info)
	return info != nil && info.Kind == model.Map
}

// IsPrimitive 检查给定的 TypeInfo 是否表示基本类型
func (c *ConcreteTypeConverter) IsPrimitive(info *model.TypeInfo) bool {
	info = c.resolveConcreteType(info)
	return info != nil && info.Kind == model.Primitive
}

// IsUltimatelyPrimitive 检查给定的 TypeInfo 或其底层类型是否最终是基本类型
func (c *ConcreteTypeConverter) IsUltimatelyPrimitive(info *model.TypeInfo) bool {
	info = c.resolveConcreteType(info)
	return info != nil && info.Kind == model.Primitive
}