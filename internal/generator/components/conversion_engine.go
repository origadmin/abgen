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
	isSliceConversion := ce.typeConverter.IsSlice(sourceInfo) && ce.typeConverter.IsSlice(targetInfo)
	slog.Debug("ConversionEngine: Initial slice check", "isSlice", isSliceConversion)

	if !isSliceConversion {
		// Check for pointer-to-slice case
		if ce.typeConverter.IsPointer(sourceInfo) && ce.typeConverter.IsPointer(targetInfo) {
			slog.Debug("ConversionEngine: Detected pointer-to-something types")
			sourceUnderlying := sourceInfo.Underlying
			targetUnderlying := targetInfo.Underlying
			if sourceUnderlying != nil && targetUnderlying != nil && ce.typeConverter.IsSlice(sourceUnderlying) && ce.typeConverter.IsSlice(targetUnderlying) {
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
	if !ce.typeConverter.IsStruct(sourceInfo) || !ce.typeConverter.IsStruct(targetInfo) {
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

	isSourcePtrToSlice := ce.typeConverter.IsPointer(sourceInfo) && ce.typeConverter.IsSlice(sourceInfo.Underlying)
	isTargetPtrToSlice := ce.typeConverter.IsPointer(targetInfo) && ce.typeConverter.IsSlice(targetInfo.Underlying)

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

	sourceElemIsPtr := ce.typeConverter.IsPointer(sourceElem)
	targetElemIsPtr := ce.typeConverter.IsPointer(targetElem)

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
		buf.WriteString("\treturn &tos\n")
	} else {
		buf.WriteString("\treturn tos\n")
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
		}

		if targetField != nil {
			conversionExpr, requiredHelpers, newTask := ce.getConversionExpression(sourceField, targetField, "from")
			allRequiredHelpers = append(allRequiredHelpers, requiredHelpers...)
			if newTask != nil {
				newTasks = append(newTasks, newTask)
			}
			fieldAssignments = append(fieldAssignments, fmt.Sprintf("\t\t%s: %s,", targetField.Name, conversionExpr))
		}
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

	convFuncName := ce.nameGenerator.ConversionFunctionName(sourceType, targetType)

	// This is the new logic to handle slice fields correctly
	isSliceConversion := (ce.typeConverter.IsSlice(sourceType) && ce.typeConverter.IsSlice(targetType)) ||
		(ce.typeConverter.IsPointer(sourceType) && ce.typeConverter.IsPointer(targetType) &&
			ce.typeConverter.IsSlice(sourceType.Underlying) &&
			ce.typeConverter.IsSlice(targetType.Underlying))

	if isSliceConversion {
		newTask := &model.ConversionTask{
			Source: sourceType,
			Target: targetType,
		}
		return fmt.Sprintf("%s(%s)", convFuncName, sourceFieldExpr), nil, newTask
	}

	sourceIsPointer := ce.typeConverter.IsPointer(sourceType)
	targetIsPointer := ce.typeConverter.IsPointer(targetType)

	newTask := &model.ConversionTask{
		Source: sourceType,
		Target: targetType,
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
