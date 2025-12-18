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
	aliasManager  model.AliasManager
	importManager model.ImportManager
}

// NewConversionEngine creates a new conversion engine.
func NewConversionEngine(
	typeConverter model.TypeConverter,
	nameGenerator model.NameGenerator,
	aliasManager model.AliasManager,
	importManager model.ImportManager,
) model.ConversionEngine {
	return &ConversionEngine{
		typeConverter: typeConverter,
		nameGenerator: nameGenerator,
		aliasManager:  aliasManager,
		importManager: importManager,
	}
}

// GenerateConversionFunction generates a conversion function.
func (ce *ConversionEngine) GenerateConversionFunction(
	sourceInfo, targetInfo *model.TypeInfo, rule *config.ConversionRule,
) (*model.GeneratedCode, error) {
	if !ce.typeConverter.IsStruct(sourceInfo) || !ce.typeConverter.IsStruct(targetInfo) {
		if ce.typeConverter.IsSlice(sourceInfo) && ce.typeConverter.IsSlice(targetInfo) {
			return ce.GenerateSliceConversion(sourceInfo, targetInfo)
		}
		return nil, nil
	}

	var buf strings.Builder
	funcName := ce.nameGenerator.GetFunctionName(sourceInfo, targetInfo)
	sourceTypeStr := ce.nameGenerator.GetTypeString(sourceInfo)
	targetTypeStr := ce.nameGenerator.GetTypeString(targetInfo)

	buf.WriteString(fmt.Sprintf("// %s converts %s to %s\n", funcName, sourceTypeStr, targetTypeStr))
	buf.WriteString(fmt.Sprintf("func %s(from *%s) *%s {\n", funcName, sourceTypeStr, targetTypeStr))
	buf.WriteString("\tif from == nil {\n\t\treturn nil\n\t}\n\n")

	structCode, requiredHelpers, err := ce.generateStructToStructConversion(sourceInfo, targetInfo, rule)
	if err != nil {
		return nil, err
	}
	buf.WriteString(structCode)

	buf.WriteString("}\n\n")

	return &model.GeneratedCode{
		FunctionBody:    buf.String(),
		RequiredHelpers: requiredHelpers,
	}, nil
}

// GenerateSliceConversion generates a slice conversion function.
func (ce *ConversionEngine) GenerateSliceConversion(
	sourceInfo, targetInfo *model.TypeInfo,
) (*model.GeneratedCode, error) {
	var buf strings.Builder
	sourceElem := ce.typeConverter.GetElementType(sourceInfo)
	targetElem := ce.typeConverter.GetElementType(targetInfo)

	ce.aliasManager.EnsureTypeAlias(sourceInfo, true)
	ce.aliasManager.EnsureTypeAlias(targetInfo, false)
	ce.aliasManager.EnsureTypeAlias(sourceElem, true)
	ce.aliasManager.EnsureTypeAlias(targetElem, false)

	sourceSliceStr := ce.nameGenerator.GetTypeString(sourceInfo)
	targetSliceStr := ce.nameGenerator.GetTypeString(targetInfo)
	funcName := ce.nameGenerator.GetFunctionName(sourceInfo, targetInfo)

	slog.Debug("Generating slice conversion", "funcName", funcName, "source", sourceSliceStr, "target", targetSliceStr)

	buf.WriteString(fmt.Sprintf("// %s converts %s to %s\n", funcName, sourceSliceStr, targetSliceStr))
	buf.WriteString(fmt.Sprintf("func %s(froms %s) %s {\n", funcName, sourceSliceStr, targetSliceStr))
	buf.WriteString("\tif froms == nil {\n\t\treturn nil\t}\n")
	buf.WriteString(fmt.Sprintf("\ttos := make(%s, len(froms))\n", targetSliceStr))
	buf.WriteString("\tfor i, f := range froms {\n")

	elemFuncName := ce.nameGenerator.GetFunctionName(sourceElem, targetElem)
	elementConversionExpr := elemFuncName + "(f)"

	buf.WriteString(fmt.Sprintf("\t\ttos[i] = %s\n", elementConversionExpr))
	buf.WriteString("\t}\n")
	buf.WriteString("\treturn tos\n")
	buf.WriteString("}\n\n")

	return &model.GeneratedCode{FunctionBody: buf.String()}, nil
}

// GetConversionExpression gets the conversion expression.
func (ce *ConversionEngine) GetConversionExpression(
	parentSource *model.TypeInfo, sourceField *model.FieldInfo,
	parentTarget *model.TypeInfo, targetField *model.FieldInfo,
	fromVar string,
) (string, bool, bool, []string) {
	sourceType := sourceField.Type
	targetType := targetField.Type
	sourceFieldExpr := fmt.Sprintf("%s.%s", fromVar, sourceField.Name)

	if sourceType.UniqueKey() == targetType.UniqueKey() {
		return sourceFieldExpr, sourceType.Kind == model.Pointer, false, nil
	}

	if ce.typeConverter.IsSlice(sourceType) && ce.typeConverter.IsSlice(targetType) {
		// ... slice handling logic ...
	}

	// Check for built-in helper functions
	helperKey, funcName := ce.findHelper(sourceType, targetType)
	if funcName != "" {
		slog.Debug("Found helper function", "key", helperKey, "funcName", funcName)
		ce.addRequiredImportsForHelper(funcName)
		isPointerReturn := targetType.Kind == model.Pointer
		return fmt.Sprintf("%s(%s)", funcName, sourceFieldExpr), isPointerReturn, true, []string{funcName}
	}

	return ce.generateFallbackConversion(sourceField, targetField, sourceFieldExpr)
}

func (ce *ConversionEngine) findHelper(source, target *model.TypeInfo) (string, string) {
	// This map should be generated or configured more robustly.
	conversionMap := map[string]string{
		"string->time.Time":                                                     "ConvertStringToTime",
		"string->github.com/google/uuid.UUID":                                   "ConvertStringToUUID",
		"time.Time->string":                                                     "ConvertTimeToString",
		"github.com/google/uuid.UUID->string":                                   "ConvertUUIDToString",
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

// generateStructToStructConversion generates a struct-to-struct conversion.
func (ce *ConversionEngine) generateStructToStructConversion(
	sourceInfo, targetInfo *model.TypeInfo, rule *config.ConversionRule,
) (string, []string, error) {
	var buf strings.Builder
	var tempVarDecls []string
	var fieldAssignments []string
	var allRequiredHelpers []string

	for _, sourceField := range sourceInfo.Fields {
		if _, shouldIgnore := rule.FieldRules.Ignore[sourceField.Name]; shouldIgnore {
			continue
		}
		targetFieldName := sourceField.Name
		if remappedName, shouldRemap := rule.FieldRules.Remap[sourceField.Name]; shouldRemap {
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
			conversionExpr, returnsPointer, isFunctionCall, requiredHelpers := ce.GetConversionExpression(
				sourceInfo, sourceField, targetInfo, targetField, "from")
			allRequiredHelpers = append(allRequiredHelpers, requiredHelpers...)

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

	if len(tempVarDecls) > 0 {
		for _, decl := range tempVarDecls {
			buf.WriteString(decl)
		}
		buf.WriteString("\n")
	}

	buf.WriteString("\tto := &" + ce.nameGenerator.GetTypeString(targetInfo) + "{\n")
	for _, assignment := range fieldAssignments {
		buf.WriteString(assignment + "\n")
	}
	buf.WriteString("\t}\n")
	buf.WriteString("\treturn to\n")

	return buf.String(), allRequiredHelpers, nil
}

// generateFallbackConversion generates a fallback conversion.
func (ce *ConversionEngine) generateFallbackConversion(
	sourceField, targetField *model.FieldInfo, sourceFieldExpr string,
) (string, bool, bool, []string) {
	sourceType := sourceField.Type
	targetType := targetField.Type

	if sourceType.Kind == model.Primitive && targetType.Kind == model.Primitive {
		return fmt.Sprintf("%s(%s)", ce.nameGenerator.GetTypeString(targetType), sourceFieldExpr), false, true, nil
	}

	funcName := ce.nameGenerator.GetFunctionName(sourceType, targetType)
	shouldReturnPointer := targetType.IsUltimatelyStruct()
	return fmt.Sprintf("%s(%s)", funcName, sourceFieldExpr), shouldReturnPointer, true, nil
}
