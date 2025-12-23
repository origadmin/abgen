package components

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

var _ model.ConversionEngine = (*ConversionEngine)(nil)

type ConversionEngine struct {
	typeConverter     model.TypeConverter
	nameGenerator     model.NameGenerator
	typeFormatter     model.TypeFormatter
	importManager     model.ImportManager
	stubsToGenerate   map[string]*model.ConversionTask
	helperMap         map[string]model.Helper
	existingFunctions map[string]bool
}

func NewConversionEngine(
	analysisResult *model.AnalysisResult,
	typeConverter model.TypeConverter,
	nameGenerator model.NameGenerator,
	typeFormatter model.TypeFormatter,
	importManager model.ImportManager,
) model.ConversionEngine {
	ce := &ConversionEngine{
		typeConverter:     typeConverter,
		nameGenerator:     nameGenerator,
		typeFormatter:     typeFormatter,
		importManager:     importManager,
		stubsToGenerate:   make(map[string]*model.ConversionTask),
		helperMap:         make(map[string]model.Helper),
		existingFunctions: analysisResult.ExistingFunctions,
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
		return ce.GenerateSliceConversion(sourceInfo, targetInfo)
	}

	if !isStructConversion {
		slog.Warn("ConversionEngine: Task is neither a struct nor a slice conversion, skipping.", "source", sourceInfo.UniqueKey())
		return nil, nil, nil
	}

	var buf strings.Builder
	funcName := ce.nameGenerator.ConversionFunctionName(sourceInfo, targetInfo)
	sourceTypeStr := ce.typeFormatter.Format(sourceInfo)
	targetTypeStr := ce.typeFormatter.Format(targetInfo)

	buf.WriteString(fmt.Sprintf("// %s converts %s to %s.\n", funcName, sourceTypeStr, targetTypeStr))
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

func (ce *ConversionEngine) GenerateSliceConversion(
	sourceInfo, targetInfo *model.TypeInfo,
) (*model.GeneratedCode, []*model.ConversionTask, error) {
	var buf strings.Builder
	var newTasks []*model.ConversionTask

	isSourcePtr := sourceInfo.Kind == model.Pointer
	isTargetPtr := targetInfo.Kind == model.Pointer

	actualSourceSlice := getConcreteType(sourceInfo)
	actualTargetSlice := getConcreteType(targetInfo)

	sourceElem := ce.typeConverter.GetSliceElementType(actualSourceSlice)
	targetElem := ce.typeConverter.GetSliceElementType(actualTargetSlice)

	if sourceElem == nil || targetElem == nil {
		return nil, nil, fmt.Errorf("could not determine element types for slice conversion from %s to %s", sourceInfo.UniqueKey(), targetInfo.UniqueKey())
	}

	sourceSliceStr := ce.typeFormatter.Format(sourceInfo)
	targetSliceStr := ce.typeFormatter.Format(targetInfo)

	targetAllocType := targetInfo
	if isTargetPtr {
		targetAllocType = getEffectiveTypeInfo(targetInfo).Underlying
	}
	targetSliceAllocStr := ce.typeFormatter.Format(targetAllocType)

	funcName := ce.nameGenerator.ConversionFunctionName(sourceInfo, targetInfo)

	sourceElemStr := ce.typeFormatter.Format(sourceElem)
	targetElemStr := ce.typeFormatter.Format(targetElem)
	buf.WriteString(fmt.Sprintf("// %s converts a slice of %s to a slice of %s.\n", funcName, sourceElemStr, targetElemStr))
	buf.WriteString(fmt.Sprintf("func %s(froms %s) %s {\n", funcName, sourceSliceStr, targetSliceStr))
	buf.WriteString("\tif froms == nil {\n\t\treturn nil\t}\n")

	loopVar := "froms"
	if isSourcePtr {
		loopVar = "(*froms)"
	}

	buf.WriteString(fmt.Sprintf("\ttos := make(%s, len(%s))\n", targetSliceAllocStr, loopVar))
	buf.WriteString(fmt.Sprintf("\tfor i, f := range %s {\n", loopVar))

	var assignment string
	if sourceElem.UniqueKey() == targetElem.UniqueKey() {
		assignment = "tos[i] = f"
	} else {
		elemFuncName := ce.nameGenerator.ConversionFunctionName(sourceElem, targetElem)
		if _, exists := ce.existingFunctions[elemFuncName]; !exists {
			elementTask := &model.ConversionTask{Source: sourceElem, Target: targetElem}
			newTasks = append(newTasks, elementTask)
		}

		arg := "f"
		if getConcreteType(sourceElem).Kind == model.Struct && sourceElem.Kind != model.Pointer {
			arg = "&f"
		}

		call := fmt.Sprintf("%s(%s)", elemFuncName, arg)

		if getConcreteType(targetElem).Kind == model.Struct && targetElem.Kind != model.Pointer {
			call = "*" + call
		}
		assignment = fmt.Sprintf("tos[i] = %s", call)
	}

	buf.WriteString(fmt.Sprintf("\t\t%s\n", assignment))
	buf.WriteString("\t}\n")

	if isTargetPtr {
		buf.WriteString(fmt.Sprintf("\treturn &tos\n"))
	} else {
		buf.WriteString(fmt.Sprintf("\treturn tos\n"))
	}

	buf.WriteString("}\n\n")

	return &model.GeneratedCode{FunctionBody: buf.String()}, newTasks, nil
}

func findField(typeInfo *model.TypeInfo, fieldName string) *model.FieldInfo {
	if typeInfo == nil {
		return nil
	}
	for _, f := range typeInfo.Fields {
		if f.Name == fieldName || strings.EqualFold(f.Name, fieldName) {
			return f
		}
	}
	return nil
}

func (ce *ConversionEngine) generateStructToStructConversion(
	sourceInfo, targetInfo *model.TypeInfo, rule *config.ConversionRule,
) (string, []model.Helper, []*model.ConversionTask, error) {
	var buf strings.Builder
	var fieldAssignments []string
	var preAssignments []string
	var allRequiredHelpers []model.Helper
	var newTasks []*model.ConversionTask

	targetTypeStr := ce.typeFormatter.Format(targetInfo)
	sourceEdgesField := findField(sourceInfo, "Edges")

	for _, targetField := range targetInfo.Fields {
		if _, shouldIgnore := rule.FieldRules.Ignore[targetField.Name]; shouldIgnore {
			continue
		}

		sourceFieldName := targetField.Name
		if remappedName, ok := rule.FieldRules.Remap[targetField.Name]; ok {
			sourceFieldName = remappedName
		}

		var sourceField *model.FieldInfo
		var sourceFieldExpr string

		// Strategy 1: Find a direct field match in the source struct.
		sourceField = findField(sourceInfo, sourceFieldName)
		if sourceField != nil {
			sourceFieldExpr = fmt.Sprintf("from.%s", sourceField.Name)
		} else if sourceEdgesField != nil {
			// Strategy 2: If no direct match, look inside the 'Edges' field.
			edgeField := findField(sourceEdgesField.Type, sourceFieldName)
			if edgeField != nil {
				sourceField = edgeField
				sourceFieldExpr = fmt.Sprintf("from.Edges.%s", edgeField.Name)
			}
		}

		if sourceField != nil {
			conversionExpr, requiredHelpers, newTask, preAssignmentsForField := ce.getConversionExpression(
				sourceField.Type,
				targetField.Type,
				sourceFieldExpr,
			)
			allRequiredHelpers = append(allRequiredHelpers, requiredHelpers...)
			if newTask != nil {
				newTasks = append(newTasks, newTask)
			}
			preAssignments = append(preAssignments, preAssignmentsForField...)

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

func (ce *ConversionEngine) getConversionExpression(
	sourceType, targetType *model.TypeInfo,
	sourceFieldExpr string,
) (string, []model.Helper, *model.ConversionTask, []string) {
	if sourceType.UniqueKey() == targetType.UniqueKey() {
		return sourceFieldExpr, nil, nil, nil
	}

	isSourcePtr := sourceType.Kind == model.Pointer
	isTargetPtr := targetType.Kind == model.Pointer
	sourceElem := ce.typeConverter.GetElementType(sourceType)
	targetElem := ce.typeConverter.GetElementType(targetType)

	isSimplePointerSwap := sourceElem.UniqueKey() == targetElem.UniqueKey() &&
		sourceElem.Kind != model.Slice && sourceElem.Kind != model.Map && sourceElem.Kind != model.Struct

	if isSimplePointerSwap {
		if !isSourcePtr && isTargetPtr {
			return "&" + sourceFieldExpr, nil, nil, nil
		}
		if isSourcePtr && !isTargetPtr {
			tempVarName := fmt.Sprintf("temp%s", strings.ReplaceAll(sourceFieldExpr, ".", ""))
			targetTypeStr := ce.typeFormatter.Format(targetType)
			preAssignment := fmt.Sprintf("\tvar %s %s\n\tif %s != nil {\n\t\t%s = *%s\n\t}", tempVarName, targetTypeStr, sourceFieldExpr, tempVarName, sourceFieldExpr)
			return tempVarName, nil, nil, []string{preAssignment}
		}
	}

	if canUseSimpleTypeConversion(sourceType, targetType) {
		targetTypeStr := ce.typeFormatter.Format(targetType)
		return fmt.Sprintf("%s(%s)", targetTypeStr, sourceFieldExpr), nil, nil, nil
	}

	if helper, found := ce.findHelper(sourceType, targetType); found {
		ce.addRequiredImportsForHelper(helper)
		return fmt.Sprintf("%s(%s)", helper.Name, sourceFieldExpr), []model.Helper{helper}, nil, nil
	}

	concreteSourceType := getConcreteType(sourceType)
	concreteTargetType := getConcreteType(targetType)
	var convFuncName string
	var newTask *model.ConversionTask

	if (concreteSourceType.Kind == model.Struct && concreteTargetType.Kind == model.Struct) ||
		(concreteSourceType.Kind == model.Slice && concreteTargetType.Kind == model.Slice) {
		convFuncName = ce.nameGenerator.ConversionFunctionName(sourceType, targetType)
		newTask = &model.ConversionTask{Source: sourceType, Target: targetType}

		if concreteSourceType.Kind == model.Struct {
			arg := sourceFieldExpr
			if sourceType.Kind != model.Pointer {
				arg = "&" + sourceFieldExpr
			}
			expr := fmt.Sprintf("%s(%s)", convFuncName, arg)
			if targetType.Kind != model.Pointer {
				expr = "*" + expr
			}
			return expr, nil, newTask, nil
		}
		return fmt.Sprintf("%s(%s)", convFuncName, sourceFieldExpr), nil, newTask, nil
	}

	// Fallback for other types, though less common for complex conversions.
	convFuncName = ce.nameGenerator.ConversionFunctionName(sourceType, targetType)
	if _, exists := ce.existingFunctions[convFuncName]; !exists {
		stubTask := &model.ConversionTask{Source: sourceType, Target: targetType}
		ce.stubsToGenerate[convFuncName] = stubTask
	}

	return fmt.Sprintf("%s(%s)", convFuncName, sourceFieldExpr), nil, nil, nil
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
