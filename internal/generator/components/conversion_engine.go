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
) (*model.GeneratedCode, error) {
	if ce.typeConverter.IsSlice(sourceInfo) && ce.typeConverter.IsSlice(targetInfo) {
		return ce.GenerateSliceConversion(sourceInfo, targetInfo)
	}

	if !ce.typeConverter.IsStruct(sourceInfo) || !ce.typeConverter.IsStruct(targetInfo) {
		return nil, nil
	}

	var buf strings.Builder
	funcName := ce.nameGenerator.ConversionFunctionName(sourceInfo, targetInfo)
	sourceTypeStr := ce.typeFormatter.Format(sourceInfo.Original.Type())
	targetTypeStr := ce.typeFormatter.Format(targetInfo.Original.Type())

	buf.WriteString(fmt.Sprintf("// %s converts %s to %s\n", funcName, sourceInfo.Name, targetInfo.Name))
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

// GenerateSliceConversion generates a function to convert a slice of one type to a slice of another.
func (ce *ConversionEngine) GenerateSliceConversion(
	sourceInfo, targetInfo *model.TypeInfo,
) (*model.GeneratedCode, error) {
	var buf strings.Builder

	sourceElem := ce.typeConverter.GetElementType(sourceInfo)
	targetElem := ce.typeConverter.GetElementType(targetInfo)

	sourceSliceStr := ce.typeFormatter.Format(sourceInfo.Original.Type())
	targetSliceStr := ce.typeFormatter.Format(targetInfo.Original.Type())
	funcName := ce.nameGenerator.ConversionFunctionName(sourceInfo, targetInfo)

	slog.Debug("Generating slice conversion", "funcName", funcName, "source", sourceSliceStr, "target", targetSliceStr)

	buf.WriteString(fmt.Sprintf("// %s converts a slice of %s to a slice of %s.\n", funcName, sourceInfo.Name, targetInfo.Name))
	buf.WriteString(fmt.Sprintf("func %s(froms %s) %s {\n", funcName, sourceSliceStr, targetSliceStr))
	buf.WriteString("\tif froms == nil {\n\t\treturn nil\t}\n")
	buf.WriteString(fmt.Sprintf("\ttos := make(%s, len(froms))\n", targetSliceStr))
	buf.WriteString("\tfor i, f := range froms {\n")

	elemFuncName := ce.nameGenerator.ConversionFunctionName(sourceElem, targetElem)
	elementConversionExpr := elemFuncName + "(&f)"

	sourceElemIsPointer := ce.typeConverter.IsPointer(sourceElem)
	targetElemIsPointer := ce.typeConverter.IsPointer(targetElem)

	if sourceElemIsPointer && !targetElemIsPointer {
		elementConversionExpr = "*" + elemFuncName + "(f)"
	} else if !sourceElemIsPointer && targetElemIsPointer {
		elementConversionExpr = elemFuncName + "(&f)"
	} else {
		elementConversionExpr = elemFuncName + "(f)"
	}

	buf.WriteString(fmt.Sprintf("\t\ttos[i] = %s\n", elementConversionExpr))
	buf.WriteString("\t}\n")
	buf.WriteString("\treturn tos\n")
	buf.WriteString("}\n\n")

	return &model.GeneratedCode{FunctionBody: buf.String()}, nil
}

// generateStructToStructConversion handles the field-by-field mapping.
func (ce *ConversionEngine) generateStructToStructConversion(
	sourceInfo, targetInfo *model.TypeInfo, rule *config.ConversionRule,
) (string, []string, error) {
	var buf strings.Builder
	var fieldAssignments []string
	var allRequiredHelpers []string

	targetTypeStr := ce.typeFormatter.Format(targetInfo.Original.Type())

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
			conversionExpr, requiredHelpers := ce.getConversionExpression(sourceField, targetField, "from")
			allRequiredHelpers = append(allRequiredHelpers, requiredHelpers...)
			fieldAssignments = append(fieldAssignments, fmt.Sprintf("\t\t%s: %s,", targetField.Name, conversionExpr))
		}
	}

	buf.WriteString(fmt.Sprintf("\tto := &%s{\n", targetTypeStr))
	for _, assignment := range fieldAssignments {
		buf.WriteString(assignment + "\n")
	}
	buf.WriteString("\t}\n")
	buf.WriteString("\treturn to\n")

	return buf.String(), allRequiredHelpers, nil
}

// getConversionExpression determines the Go expression needed to convert a source field to a target field.
func (ce *ConversionEngine) getConversionExpression(
	sourceField, targetField *model.FieldInfo, fromVar string,
) (string, []string) {
	sourceType := sourceField.Type
	targetType := targetField.Type
	sourceFieldExpr := fmt.Sprintf("%s.%s", fromVar, sourceField.Name)

	if sourceType.UniqueKey() == targetType.UniqueKey() {
		return sourceFieldExpr, nil
	}

	helperKey, funcName := ce.findHelper(sourceType, targetType)
	if funcName != "" {
		slog.Debug("Found helper function", "key", helperKey, "funcName", funcName)
		ce.addRequiredImportsForHelper(funcName)
		return fmt.Sprintf("%s(%s)", funcName, sourceFieldExpr), []string{funcName}
	}

	convFuncName := ce.nameGenerator.ConversionFunctionName(sourceType, targetType)
	sourceIsPointer := ce.typeConverter.IsPointer(sourceType)
	targetIsPointer := ce.typeConverter.IsPointer(targetType)

	if !sourceIsPointer && targetIsPointer {
		return fmt.Sprintf("%s(&%s)", convFuncName, sourceFieldExpr), nil
	}
	return fmt.Sprintf("%s(%s)", convFuncName, sourceFieldExpr), nil
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
