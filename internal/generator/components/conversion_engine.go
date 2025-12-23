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
	typeConverter     model.TypeConverter
	nameGenerator     model.NameGenerator
	typeFormatter     model.TypeFormatter
	importManager     model.ImportManager
	stubsToGenerate   map[string]*model.ConversionTask
	helperMap         map[string]model.Helper
	existingFunctions map[string]bool
}

// NewConversionEngine creates a new conversion engine.
func NewConversionEngine(
	typeConverter model.TypeConverter,
	nameGenerator model.NameGenerator,
	typeFormatter model.TypeFormatter,
	importManager model.ImportManager,
	existingFunctions map[string]bool,
) model.ConversionEngine {
	ce := &ConversionEngine{
		typeConverter:     typeConverter,
		nameGenerator:     nameGenerator,
		typeFormatter:     typeFormatter,
		importManager:     importManager,
		stubsToGenerate:   make(map[string]*model.ConversionTask),
		helperMap:         make(map[string]model.Helper),
		existingFunctions: existingFunctions,
	}
	ce.initializeHelpers()
	return ce
}

func (ce *ConversionEngine) initializeHelpers() {
	for _, h := range GetBuiltInHelpers() {
		key := h.SourceType + "->" + h.TargetType
		ce.helperMap[key] = h
	}
}

// GenerateConversionFunction generates a conversion function for structs or delegates to slice conversion.
func (ce *ConversionEngine) GenerateConversionFunction(
	sourceInfo, targetInfo *model.TypeInfo, rule *config.ConversionRule,
) (*model.GeneratedCode, []*model.ConversionTask, error) {
	slog.Debug("ConversionEngine: Received task", "source", sourceInfo.UniqueKey(), "target", targetInfo.UniqueKey())

	concreteSource := getConcreteType(sourceInfo)
	concreteTarget := getConcreteType(targetInfo)

	isSliceConversion := concreteSource.Kind == model.Slice && concreteTarget.Kind == model.Slice
	isStructConversion := concreteSource.Kind == model.Struct && concreteTarget.Kind == model.Struct

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

	isSourcePtr := sourceInfo.Kind == model.Pointer
	isTargetPtr := targetInfo.Kind == model.Pointer

	actualSourceSlice := getConcreteType(sourceInfo)
	actualTargetSlice := getConcreteType(targetInfo)

	sourceElem := ce.typeConverter.GetSliceElementType(actualSourceSlice)
	targetElem := ce.typeConverter.GetSliceElementType(actualTargetSlice)

	if sourceElem == nil || targetElem == nil {
		return nil, fmt.Errorf("could not determine element types for slice conversion from %s to %s", sourceInfo.UniqueKey(), targetInfo.UniqueKey())
	}

	sourceSliceStr := ce.typeFormatter.Format(sourceInfo)
	targetSliceStr := ce.typeFormatter.Format(targetInfo)

	targetAllocType := targetInfo
	if isTargetPtr {
		targetAllocType = getEffectiveTypeInfo(targetInfo).Underlying
	}
	targetSliceAllocStr := ce.typeFormatter.Format(targetAllocType)

	funcName := ce.nameGenerator.ConversionFunctionName(sourceInfo, targetInfo)

	slog.Debug("Generating slice conversion", "funcName", funcName, "source", sourceSliceStr, "target", targetSliceStr)

	buf.WriteString(fmt.Sprintf("// %s converts a slice of %s to a slice of %s.\n", funcName, actualSourceSlice.Name, actualTargetSlice.Name))
	buf.WriteString(fmt.Sprintf("func %s(froms %s) %s {\n", funcName, sourceSliceStr, targetSliceStr))
	buf.WriteString("\tif froms == nil {\n\t\treturn nil\t}\n")

	loopVar := "froms"
	if isSourcePtr {
		loopVar = "(*froms)"
	}

	buf.WriteString(fmt.Sprintf("\ttos := make(%s, len(%s))\n", targetSliceAllocStr, loopVar))
	buf.WriteString(fmt.Sprintf("\tfor i, f := range %s {\n", loopVar))

	elemFuncName := ce.nameGenerator.ConversionFunctionName(sourceElem, targetElem)

	arg := "f"
	if getConcreteType(sourceElem).Kind == model.Struct && sourceElem.Kind != model.Pointer {
		arg = "&f"
	}

	call := fmt.Sprintf("%s(%s)", elemFuncName, arg)

	if getConcreteType(targetElem).Kind == model.Struct && targetElem.Kind != model.Pointer {
		call = "*" + call
	}

	assignment := fmt.Sprintf("tos[i] = %s", call)

	buf.WriteString(fmt.Sprintf("\t\t%s\n", assignment))
	buf.WriteString("\t}\n")

	if isTargetPtr {
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
) (string, []model.Helper, []*model.ConversionTask, error) {
	var buf strings.Builder
	var fieldAssignments []string
	var preAssignments []string
	var allRequiredHelpers []model.Helper
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
			if strings.EqualFold(tf.Name, targetFieldName) {
				targetField = tf
				break
			}
		}

		if targetField != nil {
			conversionExpr, requiredHelpers, newTask := ce.getConversionExpression(sourceInfo, targetInfo, sourceField, targetField, "from")
			allRequiredHelpers = append(allRequiredHelpers, requiredHelpers...)
			if newTask != nil {
				newTasks = append(newTasks, newTask)
			}

			if ce.needsTemporaryVariable(sourceField.Type, targetField.Type) {
				tempVarName := fmt.Sprintf("temp%s", targetField.Name)
				preAssignments = append(preAssignments, fmt.Sprintf("\t%s := %s", tempVarName, conversionExpr))
				conversionExpr = tempVarName
				if targetField.Type.Kind == model.Pointer && getConcreteType(targetField.Type.Underlying).Kind == model.Slice {
					conversionExpr = fmt.Sprintf("&%s", conversionExpr)
				}
			}

			fieldAssignments = append(fieldAssignments, fmt.Sprintf("\t\t%s: %s,", targetField.Name, conversionExpr))
		}
	}

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

func getConcreteType(info *model.TypeInfo) *model.TypeInfo {
	if info == nil {
		return nil
	}
	if info.Kind == model.Named {
		return getConcreteType(info.Underlying)
	}
	if info.Kind == model.Pointer && info.Underlying != nil {
		return getConcreteType(info.Underlying)
	}
	return info
}

func getEffectiveTypeInfo(info *model.TypeInfo) *model.TypeInfo {
	if info == nil {
		return nil
	}
	if info.Kind == model.Named {
		return getEffectiveTypeInfo(info.Underlying)
	}
	return info
}

func (ce *ConversionEngine) getConversionExpression(
	sourceParent, targetParent *model.TypeInfo,
	sourceField, targetField *model.FieldInfo, fromVar string,
) (string, []model.Helper, *model.ConversionTask) {
	sourceType := sourceField.Type
	targetType := targetField.Type
	sourceFieldExpr := fmt.Sprintf("%s.%s", fromVar, sourceField.Name)

	if sourceType.UniqueKey() == targetType.UniqueKey() {
		return sourceFieldExpr, nil, nil
	}

	if canUseSimpleTypeConversion(sourceType, targetType) {
		targetTypeStr := ce.typeFormatter.Format(targetType)
		return fmt.Sprintf("%s(%s)", targetTypeStr, sourceFieldExpr), nil, nil
	}

	if helper, found := ce.findHelper(sourceType, targetType); found {
		ce.addRequiredImportsForHelper(helper)
		return fmt.Sprintf("%s(%s)", helper.Name, sourceFieldExpr), []model.Helper{helper}, nil
	}

	concreteSourceType := getConcreteType(sourceType)
	concreteTargetType := getConcreteType(targetType)
	var convFuncName string
	var newTask *model.ConversionTask

	if concreteSourceType.Kind == model.Struct && concreteTargetType.Kind == model.Struct {
		convFuncName = ce.nameGenerator.ConversionFunctionName(sourceType, targetType)
		newTask = &model.ConversionTask{Source: sourceType, Target: targetType}
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

	if concreteSourceType.Kind == model.Slice && concreteTargetType.Kind == model.Slice {
		isSourcePtr := sourceType.Kind == model.Pointer
		isTargetPtr := targetType.Kind == model.Pointer

		actualSourceType := sourceType
		actualTargetType := targetType
		arg := sourceFieldExpr

		if isSourcePtr && isTargetPtr {
			actualSourceType = sourceType.Underlying
			actualTargetType = targetType.Underlying
			arg = "*" + arg
		}

		convFuncName = ce.nameGenerator.ConversionFunctionName(actualSourceType, actualTargetType)
		newTask = &model.ConversionTask{Source: actualSourceType, Target: actualTargetType}
		return fmt.Sprintf("%s(%s)", convFuncName, arg), nil, newTask
	}

	if sourceType.Kind == model.Named && targetType.Kind == model.Named {
		convFuncName = ce.nameGenerator.ConversionFunctionName(sourceType, targetType)
	} else {
		convFuncName = ce.nameGenerator.FieldConversionFunctionName(sourceParent, targetParent, sourceField, targetField)
	}

	if _, exists := ce.existingFunctions[convFuncName]; !exists {
		stubTask := &model.ConversionTask{Source: sourceType, Target: targetType}
		ce.stubsToGenerate[convFuncName] = stubTask
	}

	return fmt.Sprintf("%s(%s)", convFuncName, sourceFieldExpr), nil, nil
}

func (ce *ConversionEngine) findHelper(source, target *model.TypeInfo) (model.Helper, bool) {
	key := source.UniqueKey() + "->" + target.UniqueKey()
	helper, found := ce.helperMap[key]
	return helper, found
}

func (ce *ConversionEngine) addRequiredImportsForHelper(helper model.Helper) {
	for _, pkg := range helper.Dependencies {
		ce.importManager.Add(pkg)
	}
}

func (ce *ConversionEngine) needsTemporaryVariable(sourceType, targetType *model.TypeInfo) bool {
	concreteSource := getConcreteType(sourceType)
	concreteTarget := getConcreteType(targetType)

	if concreteSource.Kind != model.Slice || concreteTarget.Kind != model.Slice {
		return false
	}
	return targetType.Kind == model.Pointer
}

func (ce *ConversionEngine) GetStubsToGenerate() map[string]*model.ConversionTask {
	return ce.stubsToGenerate
}

func canUseSimpleTypeConversion(source, target *model.TypeInfo) bool {
	sourceBase := getConcreteType(source)
	targetBase := getConcreteType(target)

	if sourceBase.Kind == model.Primitive && targetBase.Kind == model.Primitive {
		if isNumeric(sourceBase.Name) && isNumeric(targetBase.Name) {
			return true
		}
	}

	if source.Name == "string" && targetBase.Kind == model.Slice {
		if targetBase.Underlying != nil && (targetBase.Underlying.Name == "byte" || targetBase.Underlying.Name == "rune") {
			return true
		}
	}

	if target.Name == "string" && sourceBase.Kind == model.Slice {
		if sourceBase.Underlying != nil && (sourceBase.Underlying.Name == "byte" || sourceBase.Underlying.Name == "rune") {
			return true
		}
	}

	return false
}

func isNumeric(typeName string) bool {
	switch typeName {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64",
		"byte", "rune":
		return true
	default:
		return false
	}
}
