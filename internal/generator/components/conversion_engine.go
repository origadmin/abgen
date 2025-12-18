package components

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

// ConversionFunctionInfo 转换函数信息
type ConversionFunctionInfo struct {
	Name       string
	SourceInfo *model.TypeInfo
	TargetInfo *model.TypeInfo
	Rule       *config.ConversionRule
}

// ConversionEngine 实现 ConversionEngine 接口
type ConcreteConversionEngine struct {
	typeConverter     model.TypeConverter
	nameGenerator     model.NameGenerator
	aliasManager      model.AliasManager
	importManager     model.ImportManager
	conversionFuncs   map[string]string
	customStubs       map[string]string
	requiredFunctions map[string]bool
	buf               *strings.Builder
}

// NewConversionEngine 创建新的转换引擎
func NewConversionEngine(
	typeConverter model.TypeConverter,
	nameGenerator model.NameGenerator,
	aliasManager model.AliasManager,
	importManager model.ImportManager,
) model.ConversionEngine {
	return &ConcreteConversionEngine{
		typeConverter:     typeConverter,
		nameGenerator:     nameGenerator,
		aliasManager:      aliasManager,
		importManager:     importManager,
		conversionFuncs:   getBuiltinConversionFunctions(),
		customStubs:       make(map[string]string),
		requiredFunctions: make(map[string]bool),
		buf:               &strings.Builder{},
	}
}

// getBuiltinConversionFunctions 返回内置转换函数映射
func getBuiltinConversionFunctions() map[string]string {
	return map[string]string{
		"string->time.Time": "ConvertStringToTime",
		"string->uuid.UUID": "ConvertStringToUUID",
		"time.Time->string": "ConvertTimeToString",
		"uuid.UUID->string": "ConvertUUIDToString",
		// 添加新的时间戳转换函数，使用 FQN 键
		"time.Time->*google.golang.org/protobuf/types/known/timestamppb.Timestamp": "ConvertTimeToTimestamp",
		"*google.golang.org/protobuf/types/known/timestamppb.Timestamp->time.Time": "ConvertTimestampToTime",
	}
}

// GenerateConversionFunction 生成转换函数
func (ce *ConcreteConversionEngine) GenerateConversionFunction(
	sourceInfo, targetInfo *model.TypeInfo, rule *config.ConversionRule,
) error {
	// 检查是否都是结构体。如果不是，可能不希望生成基于指针的转换
	if !ce.typeConverter.IsStruct(sourceInfo) || !ce.typeConverter.IsStruct(targetInfo) {
		// 对于非结构体类型（如切片），生成基于值的转换
		if ce.typeConverter.IsSlice(sourceInfo) && ce.typeConverter.IsSlice(targetInfo) {
			return ce.GenerateSliceConversion(sourceInfo, targetInfo)
		}
		// 如有必要，在此处理其他非结构体、非切片类型
		return nil
	}

	funcName := ce.nameGenerator.GetFunctionName(sourceInfo, targetInfo)

	sourceTypeStr := ce.nameGenerator.GetTypeAliasString(sourceInfo)
	targetTypeStr := ce.nameGenerator.GetTypeAliasString(targetInfo)

	ce.buf.WriteString(fmt.Sprintf("// %s converts %s to %s\n", funcName, sourceTypeStr, targetTypeStr))
	ce.buf.WriteString(fmt.Sprintf("func %s(from *%s) *%s {\n", funcName, sourceTypeStr, targetTypeStr))
	ce.buf.WriteString("\tif from == nil {\n\t\treturn nil\n\t}\n\n")

	err := ce.generateStructToStructConversion(sourceInfo, targetInfo, rule)
	if err != nil {
		return err
	}

	ce.buf.WriteString("}\n\n")
	return nil
}

// GenerateSliceConversion 生成切片转换函数
func (ce *ConcreteConversionEngine) GenerateSliceConversion(
	sourceInfo, targetInfo *model.TypeInfo,
) error {
	sourceElem := ce.typeConverter.GetElementType(sourceInfo)
	targetElem := ce.typeConverter.GetElementType(targetInfo)

	// 确保切片类型有别名
	ce.aliasManager.EnsureTypeAlias(sourceInfo, true)
	ce.aliasManager.EnsureTypeAlias(targetInfo, false)
	ce.aliasManager.EnsureTypeAlias(sourceElem, true)
	ce.aliasManager.EnsureTypeAlias(targetElem, false)

	sourceSliceStr := ce.nameGenerator.GetTypeAliasString(sourceInfo)
	targetSliceStr := ce.nameGenerator.GetTypeAliasString(targetInfo)

	funcName := ce.nameGenerator.GetFunctionName(sourceInfo, targetInfo)

	slog.Debug("Generating slice conversion",
		"funcName", funcName,
		"sourceSliceStr", sourceSliceStr,
		"targetSliceStr", targetSliceStr)

	ce.buf.WriteString(fmt.Sprintf("// %s converts %s to %s\n", funcName, sourceSliceStr, targetSliceStr))
	ce.buf.WriteString(fmt.Sprintf("func %s(froms %s) %s {\n", funcName, sourceSliceStr, targetSliceStr))
	ce.buf.WriteString("\tif froms == nil {\n\t\treturn nil\t}\n")
	ce.buf.WriteString(fmt.Sprintf("\ttos := make(%s, len(froms))\n", targetSliceStr))
	ce.buf.WriteString("\tfor i, f := range froms {\n")

	// 获取元素转换函数名
	elemFuncName := ce.nameGenerator.GetFunctionName(sourceElem, targetElem)

	// 确定基于元素类型的转换表达式
	var elementConversionExpr string
	var returnsPointer bool

	elementConversionExpr = elemFuncName + "(f)"

	if sourceElem.Kind == model.Pointer && targetElem.Kind == model.Pointer {
		returnsPointer = true // *T -> *U returns *U
	} else if sourceElem.Kind == model.Pointer && targetElem.Kind != model.Pointer {
		returnsPointer = false // *T -> U returns U
	} else if sourceElem.Kind != model.Pointer && targetElem.Kind == model.Pointer {
		returnsPointer = false // T -> *U returns U
	} else {
		returnsPointer = true // T -> U returns U
	}

	finalExpr := elementConversionExpr
	targetIsPointer := targetElem.Kind == model.Pointer

	// 处理切片元素的指针/值转换
	if returnsPointer && !targetIsPointer {
		finalExpr = elementConversionExpr
	} else if !returnsPointer && targetIsPointer {
		// 对于返回值但目标元素是指针的情况，需要取地址
		tempVarName := "tmp"
		ce.buf.WriteString(fmt.Sprintf("\t\t%s := %s\n", tempVarName, elementConversionExpr))
		finalExpr = fmt.Sprintf("&%s", tempVarName)
	}

	ce.buf.WriteString(fmt.Sprintf("\t\ttos[i] = %s\n", finalExpr))
	ce.buf.WriteString("\t}\n")
	ce.buf.WriteString("\treturn tos\n")
	ce.buf.WriteString("}\n\n")

	return nil
}

// GetConversionExpression 获取转换表达式
func (ce *ConcreteConversionEngine) GetConversionExpression(
	parentSource *model.TypeInfo, sourceField *model.FieldInfo,
	parentTarget *model.TypeInfo, targetField *model.FieldInfo,
	fromVar string,
) (string, bool, bool) {
	sourceType := sourceField.Type
	targetType := targetField.Type
	sourceFieldExpr := fromVar
	if sourceField.Name != "" {
		sourceFieldExpr = fmt.Sprintf("%s.%s", fromVar, sourceField.Name)
	}

	slog.Debug("GetConversionExpression",
		"sourceField", sourceField.Name,
		"sourceType.Kind", sourceType.Kind,
		"sourceType.Name", sourceType.Name,
		"targetField", targetField.Name,
		"targetType.Kind", targetType.Kind,
		"targetType.Name", targetType.Name,
	)

	// 优先处理切片转换逻辑
	if sourceType.Kind == model.Slice && targetType.Kind == model.Slice {
		sourceElem := ce.typeConverter.GetElementType(sourceType)
		targetElem := ce.typeConverter.GetElementType(targetType)

		if sourceElem != nil && targetElem != nil {
			if sourceElem.UniqueKey() != targetElem.UniqueKey() {
				// 确保切片类型有别名
				ce.aliasManager.EnsureTypeAlias(sourceType, true)
				ce.aliasManager.EnsureTypeAlias(targetType, false)
				ce.aliasManager.EnsureTypeAlias(sourceElem, true)
				ce.aliasManager.EnsureTypeAlias(targetElem, false)

				sliceConverterFuncName := ce.nameGenerator.GetFunctionName(sourceType, targetType)
				ce.requiredFunctions[sliceConverterFuncName] = true

				slog.Debug("Slice conversion prepared",
					"sourceType", sourceType.TypeString(),
					"targetType", targetType.TypeString(),
					"funcName", sliceConverterFuncName)

				// 切片转换函数返回切片，不是指针
				return fmt.Sprintf("%s(%s)", sliceConverterFuncName, sourceFieldExpr), false, true
			}
		}
	}

	sourceKey := sourceType.UniqueKey()
	targetKey := targetType.UniqueKey()

	if sourceKey == targetKey {
		// 直接字段访问，返回值是指针取决于 sourceType 本身是否是指针
		return sourceFieldExpr, sourceType.Kind == model.Pointer, false
	}

	// 检查在统一的 conversionFunctions 映射中
	conversionKey := sourceKey + "->" + targetKey
	slog.Debug("GetConversionExpression", "action", "checking conversionFunctions", "conversionKey", conversionKey)
	if funcName, ok := ce.conversionFuncs[conversionKey]; ok {
		slog.Debug("GetConversionExpression", "action", "found in conversionFunctions", "funcName", funcName)
		ce.requiredFunctions[funcName] = true
		if strings.Contains(funcName, "Time") {
			ce.importManager.Add("time")
		}
		if strings.Contains(funcName, "UUID") {
			ce.importManager.Add("github.com/google/uuid")
		}
		if strings.Contains(funcName, "Timestamp") {
			ce.importManager.Add("google.golang.org/protobuf/types/known/timestamppb")
		}
		// 根据目标Key确定辅助函数是否返回指针
		isPointerReturn := targetType.Kind == model.Pointer
		return fmt.Sprintf("%s(%s)", funcName, sourceFieldExpr), isPointerReturn, true
	}

	// 其他情况的回退逻辑...
	// [这里可以添加更多的转换逻辑处理]

	return ce.generateFallbackConversion(sourceField, targetField, sourceFieldExpr)
}

// generateStructToStructConversion 生成结构体到结构体的转换
func (ce *ConcreteConversionEngine) generateStructToStructConversion(
	sourceInfo, targetInfo *model.TypeInfo, rule *config.ConversionRule,
) error {
	var tempVarDecls []string
	var fieldAssignments []string

	for _, sourceField := range sourceInfo.Fields {
		if _, shouldIgnore := rule.FieldRules.Ignore[sourceField.Name]; shouldIgnore {
			continue
		}
		targetFieldName := sourceField.Name
		if remappedName, shouldRemap := rule.FieldRules.Remap[sourceField.Name]; shouldRemap {
			targetFieldName = remappedName
		}

		var targetField *model.FieldInfo
		// 首先，尝试精确（区分大小写）匹配
		for _, tf := range targetInfo.Fields {
			if tf.Name == targetFieldName {
				targetField = tf
				break
			}
		}
		// 如果没有精确匹配，回退到不区分大小写的匹配
		if targetField == nil {
			for _, tf := range targetInfo.Fields {
				if strings.EqualFold(tf.Name, targetFieldName) {
					targetField = tf
					break
				}
			}
		}

		if targetField != nil {
			conversionExpr, returnsPointer, isFunctionCall := ce.GetConversionExpression(
				sourceInfo, sourceField, targetInfo, targetField, "from")

			finalExpr := conversionExpr
			targetIsPointer := targetField.Type.Kind == model.Pointer

			if returnsPointer && !targetIsPointer {
				finalExpr = fmt.Sprintf("*%s", conversionExpr)
			} else if !returnsPointer && targetIsPointer {
				if isFunctionCall {
					tempVarName := fmt.Sprintf("tmp%s", targetField.Name)
					tempVarDecls = append(tempVarDecls, fmt.Sprintf("\t%s := %s\n", tempVarName, conversionExpr))
					finalExpr = fmt.Sprintf("&%s", tempVarName)
				} else {
					finalExpr = fmt.Sprintf("&%s", conversionExpr)
				}
			}
			fieldAssignments = append(fieldAssignments, fmt.Sprintf("\t\t%s: %s,", targetField.Name, finalExpr))
		}
	}

	// 写入临时变量声明
	if len(tempVarDecls) > 0 {
		for _, decl := range tempVarDecls {
			ce.buf.WriteString(decl)
		}
		ce.buf.WriteString("\n") // 添加分隔符
	}

	ce.buf.WriteString("\tto := &" + ce.nameGenerator.GetTypeString(targetInfo) + "{\n")
	for _, assignment := range fieldAssignments {
		ce.buf.WriteString(assignment + "\n")
	}
	ce.buf.WriteString("\t}\n")
	ce.buf.WriteString("\treturn to\n")

	return nil
}

// generateFallbackConversion 生成回退转换
func (ce *ConcreteConversionEngine) generateFallbackConversion(
	sourceField, targetField *model.FieldInfo, sourceFieldExpr string,
) (string, bool, bool) {
	sourceType := sourceField.Type
	targetType := targetField.Type

	// 处理数字基本类型转换（例如，int 到 int32）
	if sourceType.Kind == model.Primitive && targetType.Kind == model.Primitive &&
		isNumericPrimitive(sourceType.Name) && isNumericPrimitive(targetType.Name) {
		return fmt.Sprintf("%s(%s)", ce.nameGenerator.GetTypeString(targetType), sourceFieldExpr), false, true
	}

	// 对于所有其他无法直接处理的类型，使用统一的命名规则
	if (sourceType.Kind == model.Struct || targetType.Kind == model.Struct) ||
		(sourceType.IsNamedType() && targetType.IsNamedType()) {
		funcName := ce.nameGenerator.GetFunctionName(sourceType, targetType)
		ce.requiredFunctions[funcName] = true
		shouldReturnPointer := targetType.IsUltimatelyStruct()
		return fmt.Sprintf("%s(%s)", funcName, sourceFieldExpr), shouldReturnPointer, true
	}

	// 生成基本转换存根
	stubFuncName := ce.nameGenerator.GetPrimitiveConversionStubName(
		&model.TypeInfo{}, sourceField, &model.TypeInfo{}, targetField)
	ce.requiredFunctions[stubFuncName] = true
	return fmt.Sprintf("%s(%s)", stubFuncName, sourceFieldExpr), false, true
}

// isNumericPrimitive 检查给定的基本类型是否为数字类型
func isNumericPrimitive(kind string) bool {
	switch kind {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float32", "float64",
		"complex64", "complex128":
		return true
	default:
		return false
	}
}

// GetGeneratedCode 获取生成的代码
func (ce *ConcreteConversionEngine) GetGeneratedCode() string {
	return ce.buf.String()
}

// GetRequiredFunctions 获取需要的函数集合
func (ce *ConcreteConversionEngine) GetRequiredFunctions() map[string]bool {
	return ce.requiredFunctions
}

// Reset 重置转换引擎状态
func (ce *ConcreteConversionEngine) Reset() {
	ce.buf.Reset()
	ce.requiredFunctions = make(map[string]bool)
	ce.customStubs = make(map[string]string)
}