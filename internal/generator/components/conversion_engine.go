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
	typeConverter model.TypeConverter
	nameGenerator model.NameGenerator
	typeFormatter *TypeFormatter
	importManager model.ImportManager
}

// NewConversionEngine creates a new conversion engine.
func NewConversionEngine(
	typeConverter model.TypeConverter,
	nameGenerator model.NameGenerator,
	typeFormatter *TypeFormatter,
	importManager model.ImportManager,
) model.ConversionEngine {
	return &ConversionEngine{
		typeConverter: typeConverter,
		nameGenerator: nameGenerator,
		typeFormatter: typeFormatter,
		importManager: importManager,
	}
}

// GenerateConversionFunction generates a conversion function for structs or delegates to slice conversion.
func (ce *ConversionEngine) GenerateConversionFunction(
	sourceInfo, targetInfo *model.TypeInfo, rule *config.ConversionRule,
) (*model.GeneratedCode, []*model.ConversionTask, error) {
	slog.Debug("ConversionEngine: Received task", "source", sourceInfo.UniqueKey(), "target", targetInfo.UniqueKey())

	// Determine if this is a slice conversion, including pointers to slices.
	isSliceConversion := sourceInfo.Kind == model.Slice && targetInfo.Kind == model.Slice
	slog.Debug("ConversionEngine: Initial slice check", "isSlice", isSliceConversion)

	if !isSliceConversion {
		// Check for pointer-to-slice case
		if sourceInfo.Kind == model.Pointer && targetInfo.Kind == model.Pointer {
			slog.Debug("ConversionEngine: Detected pointer-to-something types")
			sourceUnderlying := sourceInfo.Underlying
			targetUnderlying := targetInfo.Underlying
			if sourceUnderlying != nil && targetUnderlying != nil && sourceUnderlying.Kind == model.Slice && targetUnderlying.Kind == model.Slice {
				slog.Debug("ConversionEngine: Confirmed pointer-to-slice types")
				isSliceConversion = true
			} else {
				slog.Debug("ConversionEngine: Pointer types are not pointing to slices")
			}
		}
	}

	if isSliceConversion {
		slog.Debug("ConversionEngine: Dispatching to GenerateSliceConversion")
		code, err := ce.GenerateSliceConversion(sourceInfo, targetInfo)
		return code, nil, err
	}

	slog.Debug("ConversionEngine: Dispatching to Struct-to-Struct or skipping")
	if sourceInfo.Kind != model.Struct || targetInfo.Kind != model.Struct {
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

	isSourcePtrToSlice := sourceInfo.Kind == model.Pointer && sourceInfo.Underlying != nil && sourceInfo.Underlying.Kind == model.Slice
	isTargetPtrToSlice := targetInfo.Kind == model.Pointer && targetInfo.Underlying != nil && targetInfo.Underlying.Kind == model.Slice

	var actualSourceSlice, actualTargetSlice *model.TypeInfo
	if isSourcePtrToSlice {
		actualSourceSlice = sourceInfo.Underlying
	} else {
		actualSourceSlice = sourceInfo
	}
	if isTargetPtrToSlice {
		actualTargetSlice = targetInfo.Underlying
	} else {
		actualTargetSlice = targetInfo
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

	helperKey, funcName := ce.findHelper(sourceType, targetType)
	if funcName != "" {
		slog.Debug("Found helper function", "key", helperKey, "funcName", funcName)
		ce.addRequiredImportsForHelper(funcName)
		return fmt.Sprintf("%s(%s)", funcName, sourceFieldExpr), []string{funcName}, nil
	}

	// Determine the function name.
	var convFuncName string
	isPurelyPrimitive := ce.typeConverter.IsPurelyPrimitiveOrCompositeOfPrimitives(sourceType) &&
		ce.typeConverter.IsPurelyPrimitiveOrCompositeOfPrimitives(targetType)

	if isPurelyPrimitive {
		convFuncName = ce.nameGenerator.FieldConversionFunctionName(sourceParent, targetParent, sourceField, targetField)
	} else {
		convFuncName = ce.nameGenerator.ConversionFunctionName(sourceType, targetType)
	}

	// This is the new logic to handle slice fields correctly
	isSliceConversion := (sourceType.Kind == model.Slice && targetType.Kind == model.Slice) ||
		(sourceType.Kind == model.Pointer && targetType.Kind == model.Pointer &&
			sourceType.Underlying != nil && sourceType.Underlying.Kind == model.Slice &&
			targetType.Underlying != nil && targetType.Underlying.Kind == model.Slice)

	if isSliceConversion {
		// For pointer-to-slice types, we want to generate conversion functions for the slice types,
		// not the pointer types. But the field access should remain the same.
		actualSourceType := sourceType
		actualTargetType := targetType

		// Adjust the field access expression for pointer-to-slice source fields
		fieldAccessExpr := sourceFieldExpr
		if sourceType.Kind == model.Pointer && sourceType.Underlying != nil && sourceType.Underlying.Kind == model.Slice {
			actualSourceType = sourceType.Underlying
			fieldAccessExpr = fmt.Sprintf("*%s", sourceFieldExpr) // dereference pointer to slice
		}
		if targetType.Kind == model.Pointer && targetType.Underlying != nil && targetType.Underlying.Kind == model.Slice {
			actualTargetType = targetType.Underlying
		}

		newTask := &model.ConversionTask{
			Source: actualSourceType,
			Target: actualTargetType,
		}

		// Build the conversion expression
		conversionExpr := fmt.Sprintf("%s(%s)", convFuncName, fieldAccessExpr)

		return conversionExpr, nil, newTask
	}

	sourceIsPointer := sourceType.Kind == model.Pointer
	targetIsPointer := targetType.Kind == model.Pointer

	// For primitive conversions, we don't create a new task because abgen doesn't generate bodies for them.
	// We assume the user will provide the implementation.
	var newTask *model.ConversionTask
	if !isPurelyPrimitive {
		newTask = &model.ConversionTask{
			Source: sourceType,
			Target: targetType,
		}
	}

	if !sourceIsPointer && targetIsPointer {
		return fmt.Sprintf("%s(&%s)", convFuncName, sourceFieldExpr), nil, newTask
	}
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
