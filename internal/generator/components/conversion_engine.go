package components

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

var _ model.ConversionEngine = (*ConversionEngine)(nil)

// ConversionEngine implements the ConversionEngine interface.
type ConversionEngine struct {
	typeConverter   model.TypeConverter
	nameGenerator   model.NameGenerator
	typeFormatter   *TypeFormatter
	importManager   model.ImportManager
	stubsToGenerate map[string]*model.ConversionTask
}

// NewConversionEngine creates a new conversion engine.
func NewConversionEngine(
	typeConverter model.TypeConverter,
	nameGenerator model.NameGenerator,
	typeFormatter *TypeFormatter,
	importManager model.ImportManager,
) model.ConversionEngine {
	return &ConversionEngine{
		typeConverter:   typeConverter,
		nameGenerator:   nameGenerator,
		typeFormatter:   typeFormatter,
		importManager:   importManager,
		stubsToGenerate: make(map[string]*model.ConversionTask),
	}
}

// GenerateConversionFunction generates a conversion function for structs or delegates to slice conversion.
func (ce *ConversionEngine) GenerateConversionFunction(
	sourceInfo, targetInfo *model.TypeInfo, rule *config.ConversionRule,
) (*model.GeneratedCode, []*model.ConversionTask, error) {
	slog.Debug("ConversionEngine: Received task", "source", sourceInfo.UniqueKey(), "target", targetInfo.UniqueKey())

	effectiveSource := getEffectiveTypeInfo(sourceInfo)
	effectiveTarget := getEffectiveTypeInfo(targetInfo)

	isSliceConversion := effectiveSource.Kind == model.Slice && effectiveTarget.Kind == model.Slice
	isStructConversion := effectiveSource.Kind == model.Struct && effectiveTarget.Kind == model.Struct

	if isSliceConversion {
		slog.Debug("ConversionEngine: Dispatching to GenerateSliceConversion")
		code, err := ce.GenerateSliceConversion(sourceInfo, targetInfo)
		return code, nil, err
	}

	if !isStructConversion {
		slog.Warn("ConversionEngine: Task is neither a struct nor a slice conversion, skipping.", "source", sourceInfo.UniqueKey())
		return nil, nil, nil
	}

	var buf strings.Builder
	funcName := ce.nameGenerator.ConversionFunctionName(sourceInfo, targetInfo)
	sourceTypeStr := ce.typeFormatter.Format(sourceInfo)
	targetTypeStr := ce.typeFormatter.Format(targetInfo)

	buf.WriteString(fmt.Sprintf("// %s converts %s to %s\n", funcName, sourceInfo.Name, targetInfo.Name))
	buf.WriteString(fmt.Sprintf("func %s(from *%s) *%s {\n", funcName, sourceTypeStr, targetTypeStr))
	buf.WriteString("\tif from == nil {\n\t\treturn nil\n\t}\n\n")

	structCode, requiredHelpers, newTasks, err := ce.generateStructToStructConversion(sourceInfo, targetInfo, rule)
	if err != nil {
		return nil, nil, err
	}
	buf.WriteString(structCode)

	buf.WriteString("}\n\n")

	return &model.GeneratedCode{
		FunctionBody:    buf.String(),
		RequiredHelpers: requiredHelpers,
	}, newTasks, nil
}

// GenerateSliceConversion generates a function to convert a slice of one type to a slice of another.
func (ce *ConversionEngine) GenerateSliceConversion(
	sourceInfo, targetInfo *model.TypeInfo,
) (*model.GeneratedCode, error) {
	var buf strings.Builder

	isSourcePtrToSlice := sourceInfo.Kind == model.Pointer && getEffectiveTypeInfo(sourceInfo.Underlying).Kind == model.Slice
	isTargetPtrToSlice := targetInfo.Kind == model.Pointer && getEffectiveTypeInfo(targetInfo.Underlying).Kind == model.Slice

	actualSourceSlice := getEffectiveTypeInfo(sourceInfo)
	if isSourcePtrToSlice {
		actualSourceSlice = getEffectiveTypeInfo(sourceInfo.Underlying)
	}
	actualTargetSlice := getEffectiveTypeInfo(targetInfo)
	if isTargetPtrToSlice {
		actualTargetSlice = getEffectiveTypeInfo(targetInfo.Underlying)
	}

	sourceElem := ce.typeConverter.GetSliceElementType(actualSourceSlice)
	targetElem := ce.typeConverter.GetSliceElementType(actualTargetSlice)

	if sourceElem == nil || targetElem == nil {
		return nil, fmt.Errorf("could not determine element types for slice conversion from %s to %s", sourceInfo.UniqueKey(), targetInfo.UniqueKey())
	}

	sourceSliceStr := ce.typeFormatter.Format(sourceInfo)
	targetSliceStr := ce.typeFormatter.Format(targetInfo)
	targetSliceAllocStr := ce.typeFormatter.Format(actualTargetSlice)

	funcName := ce.nameGenerator.ConversionFunctionName(sourceInfo, targetInfo)

	slog.Debug("Generating slice conversion", "funcName", funcName, "source", sourceSliceStr, "target", targetSliceStr)

	buf.WriteString(fmt.Sprintf("// %s converts a slice of %s to a slice of %s.\n", funcName, actualSourceSlice.Name, actualTargetSlice.Name))
	buf.WriteString(fmt.Sprintf("func %s(froms %s) %s {\n", funcName, sourceSliceStr, targetSliceStr))
	buf.WriteString("\tif froms == nil {\n\t\treturn nil\t}\n")

	loopVar := "froms"
	if isSourcePtrToSlice {
		loopVar = "(*froms)"
	}

	buf.WriteString(fmt.Sprintf("\ttos := make(%s, len(%s))\n", targetSliceAllocStr, loopVar))
	buf.WriteString(fmt.Sprintf("\tfor i, f := range %s {\n", loopVar))

	elemFuncName := ce.nameGenerator.ConversionFunctionName(sourceElem, targetElem)

	sourceElemIsPtr := sourceElem.Kind == model.Pointer
	targetElemIsPtr := targetElem.Kind == model.Pointer

	var conversionExpr string
	if !sourceElemIsPtr && !targetElemIsPtr { // V -> V
		conversionExpr = fmt.Sprintf("tos[i] = *%s(&f)", elemFuncName)
	} else if !sourceElemIsPtr && targetElemIsPtr { // V -> *P
		conversionExpr = fmt.Sprintf("tos[i] = %s(&f)", elemFuncName)
	} else if sourceElemIsPtr && !targetElemIsPtr { // *P -> V
		conversionExpr = fmt.Sprintf("tos[i] = *%s(f)", elemFuncName)
	} else { // *P -> *P
		conversionExpr = fmt.Sprintf("tos[i] = %s(f)", elemFuncName)
	}

	buf.WriteString(fmt.Sprintf("\t\t%s\n", conversionExpr))
	buf.WriteString("\t}\n")

	if isTargetPtrToSlice {
		buf.WriteString(fmt.Sprintf("\treturn &tos\n"))
	} else {
		buf.WriteString(fmt.Sprintf("\treturn tos\n"))
	}

	buf.WriteString("}\n\n")

	return &model.GeneratedCode{FunctionBody: buf.String()}, nil
}

// generateStructToStructConversion handles the field-by-field mapping.
func (ce *ConversionEngine) generateStructToStructConversion(
	sourceInfo, targetInfo *model.TypeInfo, rule *config.ConversionRule,
) (string, []string, []*model.ConversionTask, error) {
	var buf strings.Builder
	var fieldAssignments []string
	var preAssignments []string
	var allRequiredHelpers []string
	var newTasks []*model.ConversionTask

	targetTypeStr := ce.typeFormatter.Format(targetInfo)

	for _, sourceField := range sourceInfo.Fields {
		if _, shouldIgnore := rule.FieldRules.Ignore[sourceField.Name]; shouldIgnore {
			continue
		}
		targetFieldName := sourceField.Name
		if remappedName, ok := rule.FieldRules.Remap[sourceField.Name]; ok {
			targetFieldName = remappedName
		}

		var targetField *model.FieldInfo
		for _, tf := range targetInfo.Fields {
			if tf.Name == targetFieldName {
				targetField = tf
				break
			}
			// Try case-insensitive matching if exact match fails
			if strings.EqualFold(tf.Name, targetFieldName) {
				targetField = tf
				break
			}
		}

		if targetField != nil {
			// Pass parent info to getConversionExpression
			conversionExpr, requiredHelpers, newTask := ce.getConversionExpression(sourceInfo, targetInfo, sourceField, targetField, "from")
			allRequiredHelpers = append(allRequiredHelpers, requiredHelpers...)
			if newTask != nil {
				newTasks = append(newTasks, newTask)
			}

			// Check if this is a complex slice conversion that needs a temporary variable
			if ce.needsTemporaryVariable(sourceField.Type, targetField.Type) {
				tempVarName := fmt.Sprintf("temp%s", targetField.Name)
				preAssignments = append(preAssignments, fmt.Sprintf("\t%s := %s", tempVarName, conversionExpr))
				conversionExpr = tempVarName
				if targetField.Type.Kind == model.Pointer && targetField.Type.Underlying != nil && targetField.Type.Underlying.Kind == model.Slice {
					conversionExpr = fmt.Sprintf("&%s", conversionExpr)
				}
			}
			
			fieldAssignments = append(fieldAssignments, fmt.Sprintf("\t\t%s: %s,", targetField.Name, conversionExpr))
		}
	}

	// Output pre-assignments (temporary variables for complex conversions)
	for _, preAssignment := range preAssignments {
		buf.WriteString(preAssignment + "\n")
	}
	if len(preAssignments) > 0 {
		buf.WriteString("\n")
	}

	buf.WriteString(fmt.Sprintf("\tto := &%s{\n", targetTypeStr))
	for _, assignment := range fieldAssignments {
		buf.WriteString(assignment + "\n")
	}
	buf.WriteString("\t}\n")
	buf.WriteString("\treturn to\n")

	return buf.String(), allRequiredHelpers, newTasks, nil
}

// getEffectiveTypeInfo unwraps named types and pointers to get to the underlying struct or slice.
func getEffectiveTypeInfo(info *model.TypeInfo) *model.TypeInfo {
	if info == nil {
		return nil
	}
	if info.Kind == model.Pointer || info.Kind == model.Named {
		if info.Underlying != nil {
			return getEffectiveTypeInfo(info.Underlying)
		}
	}
	return info
}

// getConversionExpression determines the Go expression needed to convert a source field to a target field.
func (ce *ConversionEngine) getConversionExpression(
	sourceParent, targetParent *model.TypeInfo,
	sourceField, targetField *model.FieldInfo, fromVar string,
) (string, []string, *model.ConversionTask) {
	sourceType := sourceField.Type
	targetType := targetField.Type
	sourceFieldExpr := fmt.Sprintf("%s.%s", fromVar, sourceField.Name)

	if sourceType.UniqueKey() == targetType.UniqueKey() {
		return sourceFieldExpr, nil, nil
	}

	// Handle direct primitive conversions first.
	if canUseSimpleTypeConversion(sourceType, targetType) {
		targetTypeStr := ce.typeFormatter.Format(targetType)
		return fmt.Sprintf("%s(%s)", targetTypeStr, sourceFieldExpr), nil, nil
	}

	helperKey, funcName := ce.findHelper(sourceType, targetType)
	if funcName != "" {
		slog.Debug("Found helper function", "key", helperKey, "funcName", funcName)
		ce.addRequiredImportsForHelper(funcName)
		return fmt.Sprintf("%s(%s)", funcName, sourceFieldExpr), []string{funcName}, nil
	}

	// Unwrap named types to check their underlying kind (struct, slice, etc.)
	effectiveSourceType := getEffectiveTypeInfo(sourceType)
	effectiveTargetType := getEffectiveTypeInfo(targetType)

	isStructConv := effectiveSourceType.Kind == model.Struct && effectiveTargetType.Kind == model.Struct
	isSliceConv := effectiveSourceType.Kind == model.Slice && effectiveTargetType.Kind == model.Slice

	var convFuncName string
	var newTask *model.ConversionTask

	if isStructConv || isSliceConv {
		convFuncName = ce.nameGenerator.ConversionFunctionName(sourceType, targetType)
		newTask = &model.ConversionTask{
			Source: sourceType,
			Target: targetType,
		}
	} else {
		// This is for primitives, named primitives, etc., that need a stub.
		convFuncName = ce.nameGenerator.FieldConversionFunctionName(sourceParent, targetParent, sourceField, targetField)
		stubTask := &model.ConversionTask{
			Source: sourceType,
			Target: targetType,
		}
		ce.stubsToGenerate[convFuncName] = stubTask
		// For stubs, we don't create a new task for the main worklist.
	}

	// Handle pointer/value mismatches for struct-to-struct field conversions.
	if isStructConv {
		arg := sourceFieldExpr
		if sourceType.Kind != model.Pointer {
			arg = fmt.Sprintf("&%s", sourceFieldExpr)
		}

		expr := fmt.Sprintf("%s(%s)", convFuncName, arg)

		if targetType.Kind != model.Pointer {
			expr = fmt.Sprintf("*%s", expr)
		}
		return expr, nil, newTask
	}

	// Default behavior for slices and stubs.
	// Assumes the generated function/stub handles pointer/value logic internally or has a matching signature.
	return fmt.Sprintf("%s(%s)", convFuncName, sourceFieldExpr), nil, newTask
}

func (ce *ConversionEngine) findHelper(source, target *model.TypeInfo) (string, string) {
	conversionMap := map[string]string{
		"string->time.Time":                   "ConvertStringToTime",
		"string->github.com/google/uuid.UUID": "ConvertStringToUUID",
		"time.Time->string":                   "ConvertTimeToString",
		"github.com/google/uuid.UUID->string": "ConvertUUIDToString",
		"time.Time->*google.golang.org/protobuf/types/known/timestamppb.Timestamp": "ConvertTimeToTimestamp",
		"*google.golang.org/protobuf/types/known/timestamppb.Timestamp->time.Time": "ConvertTimestampToTime",
	}
	key := source.UniqueKey() + "->" + target.UniqueKey()
	if name, ok := conversionMap[key]; ok {
		return key, name
	}
	return "", ""
}

func (ce *ConversionEngine) addRequiredImportsForHelper(funcName string) {
	if strings.Contains(funcName, "Time") {
		ce.importManager.Add("time")
	}
	if strings.Contains(funcName, "UUID") {
		ce.importManager.Add("github.com/google/uuid")
	}
	if strings.Contains(funcName, "Timestamp") {
		ce.importManager.Add("google.golang.org/protobuf/types/known/timestamppb")
	}
}

// needsTemporaryVariable determines if a conversion needs a temporary variable
func (ce *ConversionEngine) needsTemporaryVariable(sourceType, targetType *model.TypeInfo) bool {
	// We need a temporary variable when converting slice types where the target is a pointer-to-slice
	// because we can't take the address of a function return value directly in a struct literal

	// Check if this is a slice conversion
	isSliceConversion := (sourceType.Kind == model.Slice && targetType.Kind == model.Slice) ||
		(sourceType.Kind == model.Pointer && targetType.Kind == model.Pointer &&
			sourceType.Underlying != nil && sourceType.Underlying.Kind == model.Slice &&
			targetType.Underlying != nil && targetType.Underlying.Kind == model.Slice)

	if !isSliceConversion {
		return false
	}

	// If target is pointer-to-slice, we need a temporary variable
	return targetType.Kind == model.Pointer && targetType.Underlying != nil && targetType.Underlying.Kind == model.Slice
}

// GetStubsToGenerate returns the stubs that need to be generated.
func (ce *ConversionEngine) GetStubsToGenerate() map[string]*model.ConversionTask {
	return ce.stubsToGenerate
}

// canUseSimpleTypeConversion checks if a simple Go type conversion T(v) is valid.
func canUseSimpleTypeConversion(source, target *model.TypeInfo) bool {
	// Get the underlying primitive type for named types
	sourceBase := source
	if source.Kind == model.Named && source.Underlying != nil {
		sourceBase = source.Underlying
	}
	targetBase := target
	if target.Kind == model.Named && target.Underlying != nil {
		targetBase = target.Underlying
	}

	// Case 1: Numeric <-> Numeric
	isSourceNumeric := sourceBase.Kind == model.Primitive && isNumeric(sourceBase.Name)
	isTargetNumeric := targetBase.Kind == model.Primitive && isNumeric(targetBase.Name)
	if isSourceNumeric && isTargetNumeric {
		return true
	}

	// Case 2: string -> []byte or []rune
	if source.Kind == model.Primitive && source.Name == "string" {
		if target.Kind == model.Slice && target.Underlying != nil {
			if target.Underlying.Name == "byte" || target.Underlying.Name == "rune" {
				return true
			}
		}
	}

	// Case 3: []byte or []rune -> string
	if target.Kind == model.Primitive && target.Name == "string" {
		if source.Kind == model.Slice && source.Underlying != nil {
			if source.Underlying.Name == "byte" || source.Underlying.Name == "rune" {
				return true
			}
		}
	}

	return false
}

// isNumeric checks if a given type name is a numeric type.
func isNumeric(typeName string) bool {
	switch typeName {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64",
		"byte", "rune": // byte is alias for uint8, rune is alias for int32
		return true
	default:
		return false
	}
}
