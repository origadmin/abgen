package generator

import (
	"bytes"
	"fmt"
	"go/format"
	"log/slog"
	"sort"
	"strings"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

var simpleConverters = map[string]string{
	// Removed all entries from simpleConverters.
}

var conversionFunctions = map[string]string{ // Renamed from helperBasedConverters
	"string->time.Time": "ConvertStringToTime",
	"string->uuid.UUID": "ConvertStringToUUID",
	"time.Time->string": "ConvertTimeToString",
	"uuid.UUID->string": "ConvertUUIDToString",
	// Add new timestamp conversion functions with FQN keys
	"time.Time->*google.golang.org/protobuf/types/known/timestamppb.Timestamp": "ConvertTimeToTimestamp",
	"*google.golang.org/protobuf/types/known/timestamppb.Timestamp->time.Time": "ConvertTimestampToTime",
}

var conversionFunctionBodies = map[string]string{ // Renamed from helperBasedConverters
	"ConvertStringToTime": `
func ConvertStringToTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
`,
	"ConvertStringToUUID": `
func ConvertStringToUUID(s string) uuid.UUID {
	u, _ := uuid.Parse(s)
	return u
}
`,
	"ConvertTimeToString": `
func ConvertTimeToString(t time.Time) string {
	return t.Format(time.RFC3339)
}
`,
	"ConvertUUIDToString": `
func ConvertUUIDToString(u uuid.UUID) string {
	return u.String()
}
`,
	// Add new timestamp conversion function bodies
	"ConvertTimeToTimestamp": `
func ConvertTimeToTimestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
`,
	"ConvertTimestampToTime": `
func ConvertTimestampToTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}
`,
}

type LegacyGenerator struct {
	config                      *config.Config
	buf                         bytes.Buffer
	importMgr                   *ImportManager
	converter                   *TypeConverter
	namer                       *Namer
	aliasMap                    map[string]string
	requiredAliases             map[string]struct{} // 新增：存储所有需要生成别名声明的 TypeInfo.UniqueKey()
	requiredConversionFunctions map[string]bool
	typeInfos                   map[string]*model.TypeInfo
	customStubs                 map[string]string
	involvedPackages            map[string]struct{}
}

func NewLegacyGenerator(config *config.Config) *LegacyGenerator {
	g := &LegacyGenerator{
		config:                      config,
		importMgr:                   NewImportManager(),
		aliasMap:                    make(map[string]string),
		requiredAliases:             make(map[string]struct{}),
		requiredConversionFunctions: make(map[string]bool),
		customStubs:                 make(map[string]string),
		involvedPackages:            make(map[string]struct{}),
	}
	g.converter = NewTypeConverter()
	g.namer = NewNamer(config, g.aliasMap)
	return g
}

// NewUnifiedGenerator 创建使用新架构的生成器
func NewUnifiedGenerator(config *config.Config, typeInfos map[string]*model.TypeInfo) model.CodeGenerator {
	return NewCodeGenerator(config, typeInfos)
}

func (g *LegacyGenerator) Generate(typeInfos map[string]*model.TypeInfo) ([]byte, error) {
	g.typeInfos = typeInfos
	slog.Debug("Generating code", "type_count", len(g.typeInfos), "initial_rules", len(g.config.ConversionRules))

	// Populate the set of involved packages early on.
	for _, pkgPath := range g.config.RequiredPackages() {
		g.involvedPackages[pkgPath] = struct{}{}
	}

	// If no explicit conversion rules are defined, discover them from package pairs.
	// This is the crucial step to enable `pair:packages` to work.
	if len(g.config.ConversionRules) == 0 {
		g.discoverImplicitConversionRules()
		slog.Debug("Implicit rule discovery finished", "discovered_rules", len(g.config.ConversionRules))
	}

	// Now that all rules are discovered, populate the namer's source package map.
	g.namer.PopulateSourcePkgs(g.config)

	// Intelligent default suffixes: Only apply if no explicit naming rules are set AND
	// if there's at least one conversion rule with ambiguous (identical) base names.
	if g.config.NamingRules.SourcePrefix == "" && g.config.NamingRules.SourceSuffix == "" &&
		g.config.NamingRules.TargetPrefix == "" && g.config.NamingRules.TargetSuffix == "" {

		needsDisambiguation := false
		for _, rule := range g.config.ConversionRules {
			sourceBaseName := ""
			if lastDot := strings.LastIndex(rule.SourceType, "."); lastDot != -1 {
				sourceBaseName = rule.SourceType[lastDot+1:]
			}

			targetBaseName := ""
			if lastDot := strings.LastIndex(rule.TargetType, "."); lastDot != -1 {
				targetBaseName = rule.TargetType[lastDot+1:]
			}

			if sourceBaseName != "" && sourceBaseName == targetBaseName {
				needsDisambiguation = true
				break
			}
		}

		if needsDisambiguation {
			slog.Debug("Ambiguous type names found with no explicit naming rules. Applying default 'Source'/'Target' suffixes.")
			g.config.NamingRules.SourceSuffix = "Source"
			g.config.NamingRules.TargetSuffix = "Target"
		}
	}

	g.populateAliases()

	var bodyBuf bytes.Buffer
	g.buf = bodyBuf

	g.writeAliases()
	g.writeConversionFunctions()
	g.writeHelperFunctions()

	finalBuf := new(bytes.Buffer)
	g.writeHeaderAndPackageToBuffer(finalBuf)
	g.writeImportsToBuffer(finalBuf)
	finalBuf.Write(g.buf.Bytes())

	return format.Source(finalBuf.Bytes())
}

// CustomStubs returns the generated custom conversion stub functions.
func (g *LegacyGenerator) CustomStubs() []byte {
	if len(g.customStubs) == 0 {
		return nil
	}

	var buf bytes.Buffer
	buf.WriteString("//go:build !abgen_source\n")
	buf.WriteString("// Code generated by abgen. DO NOT EDIT.\n")
	buf.WriteString(fmt.Sprintf("// versions: %s\n", "abgen"))
	buf.WriteString(fmt.Sprintf("// source: %s\n\n", g.config.GenerationContext.DirectivePath))
	buf.WriteString(fmt.Sprintf("package %s\n\n", g.getPackageName()))

	g.importMgr.WriteImportsToBuffer(&buf)

	g.writeCustomStubsToBuffer(&buf)

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		slog.Error("Failed to format custom stubs", "error", err)
		return buf.Bytes()
	}
	return formatted
}

func (g *LegacyGenerator) writeHeaderAndPackageToBuffer(buf *bytes.Buffer) {
	pkgName := g.getPackageName()
	buf.WriteString("//go:build !abgen_source\n")
	buf.WriteString("// Code generated by abgen. DO NOT EDIT.\n")
	buf.WriteString(fmt.Sprintf("// versions: %s\n", "abgen"))
	buf.WriteString(fmt.Sprintf("// source: %s\n\n", g.config.GenerationContext.DirectivePath))
	buf.WriteString(fmt.Sprintf("package %s\n\n", pkgName))
}

func (g *LegacyGenerator) writeImportsToBuffer(buf *bytes.Buffer) {
	imports := g.importMgr.GetAllImports()
	if len(imports) == 0 {
		return
	}
	buf.WriteString("import (\n")
	sort.Strings(imports)
	for _, importPath := range imports {
		alias := g.importMgr.GetAlias(importPath)
		buf.WriteString(fmt.Sprintf("\t%s %q\n", alias, importPath))
	}
	buf.WriteString(")\n\n")
}

func (g *LegacyGenerator) writeAliases() {
	type aliasPair struct {
		aliasName, fqn string
	}
	aliasesToWrite := make([]aliasPair, 0)
	for fqn, alias := range g.aliasMap {
		// Only generate alias declarations when fqn exists in g.requiredAliases
		if _, ok := g.requiredAliases[fqn]; !ok {
			continue
		}
		// Skip aliases that are already defined in the config (ExistingAliases)
		if _, exists := g.config.ExistingAliases[alias]; exists {
			continue
		}
		aliasesToWrite = append(aliasesToWrite, aliasPair{alias, fqn})
	}

	// Filter out aliases for types that were not actually resolved (e.g., due to errors)
	validAliases := make([]aliasPair, 0)
	for _, alias := range aliasesToWrite {
		if g.typeInfos[alias.fqn] != nil {
			validAliases = append(validAliases, alias)
		}
	}

	if len(validAliases) == 0 {
		return
	}

	sort.Slice(validAliases, func(i, j int) bool { return validAliases[i].aliasName < validAliases[j].aliasName })

	g.buf.WriteString("// Local type aliases for external types.\n")
	g.buf.WriteString("type (\n")

	for _, item := range validAliases {
		typeInfo := g.typeInfos[item.fqn]
		// Use g.namer.GetTypeString to get the original type name
		originalTypeName := g.namer.GetTypeString(typeInfo)
		g.buf.WriteString(fmt.Sprintf("\t%s = %s\n", item.aliasName, originalTypeName))
	}
	g.buf.WriteString(")\n\n")
}

func (g *LegacyGenerator) writeConversionFunctions() {
	// 1. 首先生成配置规则中的转换函数
	rules := g.config.ConversionRules
	sort.Slice(rules, func(i, j int) bool { return rules[i].SourceType < rules[j].SourceType })

	for _, rule := range rules {
		sourceInfo := g.typeInfos[rule.SourceType]
		targetInfo := g.typeInfos[rule.TargetType]
		if sourceInfo == nil || targetInfo == nil {
			slog.Warn("Skipping conversion rule due to unresolved type", "sourceType", rule.SourceType, "targetType", rule.TargetType)
			continue
		}
		g.generateConversionFunction(sourceInfo, targetInfo, rule)
	}

	// 2. 然后生成动态发现的转换函数（如切片转换函数）
	// 直接使用我们之前记录的函数名，通过重新调用getConversionExpression来找到对应的类型信息
	g.generateDynamicConversionFunctions()
}

// generateDynamicConversionFunctions 生成动态发现的转换函数
func (g *LegacyGenerator) generateDynamicConversionFunctions() {
	// 收集所有需要切片转换的类型对
	sliceConversions := make(map[string]struct {
		sourceInfo *model.TypeInfo
		targetInfo *model.TypeInfo
		funcName   string
	})

	// 扫描所有结构体的字段，找到需要切片转换的地方
	for _, rule := range g.config.ConversionRules {
		sourceInfo := g.typeInfos[rule.SourceType]
		targetInfo := g.typeInfos[rule.TargetType]
		if sourceInfo == nil || targetInfo == nil {
			continue
		}

		// 检查字段中的切片类型
		for _, sourceField := range sourceInfo.Fields {
			var targetField *model.FieldInfo
			// 找到对应的目标字段
			for _, tf := range targetInfo.Fields {
				if tf.Name == sourceField.Name {
					targetField = tf
					break
				}
			}

			if targetField != nil &&
				g.converter.IsSlice(sourceField.Type) &&
				g.converter.IsSlice(targetField.Type) {

				sourceElem := g.converter.GetElementType(sourceField.Type)
				targetElem := g.converter.GetElementType(targetField.Type)

				if sourceElem != nil && targetElem != nil && sourceElem.UniqueKey() != targetElem.UniqueKey() {
					funcName := g.namer.GetFunctionName(sourceField.Type, targetField.Type)
					key := sourceField.Type.UniqueKey() + "->" + targetField.Type.UniqueKey()
					sliceConversions[key] = struct {
						sourceInfo *model.TypeInfo
						targetInfo *model.TypeInfo
						funcName   string
					}{
						sourceInfo: sourceField.Type,
						targetInfo: targetField.Type,
						funcName:   funcName,
					}
				}
			}
		}
	}

	// 生成切片转换函数
	for _, conversion := range sliceConversions {
		slog.Debug("Generating slice conversion function",
			"funcName", conversion.funcName,
			"sourceType", conversion.sourceInfo.TypeString(),
			"targetType", conversion.targetInfo.TypeString())
		g.generateSliceToSliceConversion(conversion.funcName, conversion.sourceInfo, conversion.targetInfo)
	}
}

func (g *LegacyGenerator) writeHelperFunctions() {
	if len(g.requiredConversionFunctions) == 0 {
		return
	}
	helpers := make([]string, 0, len(g.requiredConversionFunctions))
	for name := range g.requiredConversionFunctions {
		helpers = append(helpers, name)
	}
	sort.Strings(helpers)
	g.buf.WriteString("\n// --- Helper Functions ---\n")
	for _, name := range helpers {
		if body, ok := conversionFunctionBodies[name]; ok {
			g.buf.WriteString(body)
		}
	}
}

func (g *LegacyGenerator) writeCustomStubsToBuffer(buf *bytes.Buffer) {
	if len(g.customStubs) == 0 {
		return
	}
	buf.WriteString("\n// --- Custom Conversion Stubs ---\n")
	stubNames := make([]string, 0, len(g.customStubs))
	for name := range g.customStubs {
		stubNames = append(stubNames, name)
	}
	sort.Strings(stubNames)

	for _, name := range stubNames {
		buf.WriteString(g.customStubs[name])
		buf.WriteString("\n")
	}
}

func (g *LegacyGenerator) generateConversionFunction(sourceInfo, targetInfo *model.TypeInfo, rule *config.ConversionRule) {
	g.doGenerateConversionFunction(sourceInfo, targetInfo, rule, false)
	if rule.Direction == config.DirectionBoth {
		reverseRule := &config.ConversionRule{
			SourceType: targetInfo.FQN(),
			TargetType: sourceInfo.FQN(),
			Direction:  config.DirectionOneway,
			FieldRules: config.FieldRuleSet{Ignore: make(map[string]struct{}), Remap: make(map[string]string)},
		}
		for from, to := range rule.FieldRules.Remap {
			reverseRule.FieldRules.Remap[to] = from
		}
		g.doGenerateConversionFunction(targetInfo, sourceInfo, reverseRule, true)
	}
}

func (g *LegacyGenerator) doGenerateConversionFunction(sourceInfo, targetInfo *model.TypeInfo, rule *config.ConversionRule, isReverse bool) {
	// Check if both types are structs. If not, we might not want to generate a pointer-based conversion.
	if !g.converter.IsStruct(sourceInfo) || !g.converter.IsStruct(targetInfo) {
		// For non-struct types (like slices), we generate a value-based conversion.
		if g.converter.IsSlice(sourceInfo) && g.converter.IsSlice(targetInfo) {
			funcName := g.namer.GetFunctionName(sourceInfo, targetInfo)
			g.generateSliceToSliceConversion(funcName, sourceInfo, targetInfo)
		}
		// Potentially handle other non-struct, non-slice types here if necessary.
		return
	}

	funcName := g.namer.GetFunctionName(sourceInfo, targetInfo)

	sourceTypeStr := g.namer.GetTypeAliasString(sourceInfo)
	targetTypeStr := g.namer.GetTypeAliasString(targetInfo)

	g.buf.WriteString(fmt.Sprintf("// %s converts %s to %s\n", funcName, sourceTypeStr, targetTypeStr))
	g.buf.WriteString(fmt.Sprintf("func %s(from *%s) *%s {\n", funcName, sourceTypeStr, targetTypeStr))
	g.buf.WriteString("\tif from == nil {\n\t\treturn nil\n\t}\n\n")
	g.generateStructToStructConversion(sourceInfo, targetInfo, rule)
	g.buf.WriteString("}\n\n")
}

func (g *LegacyGenerator) generateSliceToSliceConversion(funcName string, sourceInfo, targetInfo *model.TypeInfo) {
	sourceElem := g.converter.GetElementType(sourceInfo)
	targetElem := g.converter.GetElementType(targetInfo)

	// 🔧 关键修复：确保切片类型在 writeAliases() 之前就被添加到 requiredAliases
	// 这样 writeAliases() 就会生成这些类型的别名声明
	g.requiredAliases[sourceInfo.UniqueKey()] = struct{}{}
	g.requiredAliases[targetInfo.UniqueKey()] = struct{}{}
	g.requiredAliases[sourceElem.UniqueKey()] = struct{}{}
	g.requiredAliases[targetElem.UniqueKey()] = struct{}{}

	// 确保别名映射中存在这些类型
	if _, exists := g.aliasMap[sourceInfo.UniqueKey()]; !exists {
		g.aliasMap[sourceInfo.UniqueKey()] = g.namer.GetAlias(sourceInfo, true)
	}
	if _, exists := g.aliasMap[targetInfo.UniqueKey()]; !exists {
		g.aliasMap[targetInfo.UniqueKey()] = g.namer.GetAlias(targetInfo, false)
	}
	if _, exists := g.aliasMap[sourceElem.UniqueKey()]; !exists {
		g.aliasMap[sourceElem.UniqueKey()] = g.namer.GetAlias(sourceElem, true)
	}
	if _, exists := g.aliasMap[targetElem.UniqueKey()]; !exists {
		g.aliasMap[targetElem.UniqueKey()] = g.namer.GetAlias(targetElem, false)
	}

	sourceSliceStr := g.namer.GetTypeAliasString(sourceInfo)
	targetSliceStr := g.namer.GetTypeAliasString(targetInfo)

	slog.Debug("Generating slice conversion",
		"funcName", funcName,
		"sourceSliceStr", sourceSliceStr,
		"targetSliceStr", targetSliceStr)

	g.buf.WriteString(fmt.Sprintf("// %s converts %s to %s\n", funcName, sourceSliceStr, targetSliceStr))
	g.buf.WriteString(fmt.Sprintf("func %s(froms %s) %s {\n", funcName, sourceSliceStr, targetSliceStr))
	g.buf.WriteString("\tif froms == nil {\n\t\treturn nil\t}\n")
	g.buf.WriteString(fmt.Sprintf("\ttos := make(%s, len(froms))\n", targetSliceStr))
	g.buf.WriteString("\tfor i, f := range froms {\n")

	// For slice element conversion, we need to determine the correct conversion function
	// and handle pointer/value semantics correctly

	// Get the element conversion function name
	elemFuncName := g.namer.GetFunctionName(sourceElem, targetElem)

	// Determine the conversion expression based on element types
	var elementConversionExpr string
	var returnsPointer bool
	var isFunctionCall bool = true

	// For any conversion, the function name is elemFuncName + "(f)"
	// The key is to understand what this function returns:
	// - If both source and target are pointers (*T -> *U), the function returns *U (pointer)
	// - If source is pointer, target is value (*T -> U), the function returns U (value) 
	// - If source is value, target is pointer (T -> *U), the function returns U (value)
	// - If both are values (T -> U), the function returns U (value)

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
	sourceIsPointer := sourceElem.Kind == model.Pointer

	// Handle pointer/value conversion for slice elements
	if returnsPointer && !targetIsPointer {
		// Case: Conversion returns a pointer, but target element is a value (e.g., []*T -> []T)
		// We need to dereference.
		// However, if the conversion function itself returns the correct type for direct assignment,
		// we should use it directly without dereferencing.
		if isFunctionCall {
			// For function calls that return pointers, assign directly without dereferencing
			// since the function should return the correct type for slice element assignment
			finalExpr = elementConversionExpr
		} else {
			// For non-function calls that return pointers, dereference
			finalExpr = fmt.Sprintf("*%s", elementConversionExpr)
		}
	} else if !returnsPointer && targetIsPointer {
		// Case: Conversion returns a value, but target element is a pointer (e.g., []T -> []*T)
		// We need to take the address.
		if isFunctionCall {
			// If it's a function call returning a value, we need a temporary variable
			// to be able to take its address.
			tempVarName := "tmp" // Scope is limited to the loop iteration
			g.buf.WriteString(fmt.Sprintf("\t\t%s := %s\n", tempVarName, elementConversionExpr))
			finalExpr = fmt.Sprintf("&%s", tempVarName)
		} else {
			// If it's a simple value (e.g., a field access), we can take its address directly.
			finalExpr = fmt.Sprintf("&%s", elementConversionExpr)
		}
	} else if sourceIsPointer && targetIsPointer && !returnsPointer {
		// Edge Case: []*T -> []*U, but the T->U conversion returns U (a value).
		// We need to take the address of the result.
		if isFunctionCall {
			tempVarName := "tmp"
			g.buf.WriteString(fmt.Sprintf("\t\t%s := %s\n", tempVarName, elementConversionExpr))
			finalExpr = fmt.Sprintf("&%s", tempVarName)
		} else {
			finalExpr = fmt.Sprintf("&%s", elementConversionExpr)
		}
	}
	// Default cases (e.g., []*T -> []*U where T->U returns *U, or []T -> []U where T->U returns U)
	// are handled correctly by `finalExpr = elementConversionExpr`.

	// The assignment logic needs to be separate from the expression generation.
	if strings.HasPrefix(finalExpr, "&") || (isFunctionCall && !returnsPointer && targetIsPointer) {
		// If we already generated a temp variable and are assigning its address,
		// the assignment is the final expression itself.
		g.buf.WriteString(fmt.Sprintf("\t\ttos[i] = %s\n", finalExpr))
	} else if isFunctionCall && !returnsPointer && targetIsPointer {
		// This case is handled above with the temp variable, but as a fallback.
		g.buf.WriteString(fmt.Sprintf("\t\ttos[i] = %s\n", finalExpr))
	} else {
		// Standard assignment.
		g.buf.WriteString(fmt.Sprintf("\t\ttos[i] = %s\n", finalExpr))
	}

	g.buf.WriteString("\t}\n")
	g.buf.WriteString("\treturn tos\n")
	g.buf.WriteString("}\n\n")
}

func (g *LegacyGenerator) generateStructToStructConversion(sourceInfo, targetInfo *model.TypeInfo, rule *config.ConversionRule) {
	// Collect temporary variable declarations
	var tempVarDecls []string
	var fieldAssignments []string // To store "FieldName: finalExpr,"

	for _, sourceField := range sourceInfo.Fields {
		if _, shouldIgnore := rule.FieldRules.Ignore[sourceField.Name]; shouldIgnore {
			continue
		}
		targetFieldName := sourceField.Name
		if remappedName, shouldRemap := rule.FieldRules.Remap[sourceField.Name]; shouldRemap {
			targetFieldName = remappedName
		}

		var targetField *model.FieldInfo
		// First, try for an exact (case-sensitive) match.
		for _, tf := range targetInfo.Fields {
			if tf.Name == targetFieldName {
				targetField = tf
				break
			}
		}
		// If no exact match, fall back to case-insensitive matching.
		if targetField == nil {
			for _, tf := range targetInfo.Fields {
				if strings.EqualFold(tf.Name, targetFieldName) {
					targetField = tf
					break
				}
			}
		}

		if targetField != nil {
			conversionExpr, returnsPointer, isFunctionCall := g.getConversionExpression(sourceInfo, sourceField, targetInfo, targetField, "from")

			finalExpr := conversionExpr
			targetIsPointer := targetField.Type.Kind == model.Pointer

			if returnsPointer && !targetIsPointer {
				// Conversion returns a pointer, but target field is not a pointer.
				// We need to dereference.
				finalExpr = fmt.Sprintf("*%s", conversionExpr)
			} else if !returnsPointer && targetIsPointer {
				// Conversion returns a value, but target field is a pointer.
				// We need to take the address.
				if isFunctionCall {
					// If it's a function call returning a value, we need a temporary variable.
					tempVarName := fmt.Sprintf("tmp%s", targetField.Name) // Generate a unique temp var name
					tempVarDecls = append(tempVarDecls, fmt.Sprintf("\t%s := %s\n", tempVarName, conversionExpr))
					finalExpr = fmt.Sprintf("&%s", tempVarName)
				} else {
					// If it's a simple value (e.g., from.Field), we can take its address directly.
					finalExpr = fmt.Sprintf("&%s", conversionExpr)
				}
			}
			// If both are pointers or both are values, no extra operation needed.
			fieldAssignments = append(fieldAssignments, fmt.Sprintf("\t\t%s: %s,", targetField.Name, finalExpr))
		}
	}

	// Write temporary variable declarations
	if len(tempVarDecls) > 0 {
		for _, decl := range tempVarDecls {
			g.buf.WriteString(decl)
		}
		g.buf.WriteString("\n") // Add a newline for separation
	}

	g.buf.WriteString("\tto := &" + g.namer.GetTypeString(targetInfo) + "{\n")
	for _, assignment := range fieldAssignments {
		g.buf.WriteString(assignment + "\n")
	}
	g.buf.WriteString("\t}\n")
	g.buf.WriteString("\treturn to\n")
}

// isNumericPrimitive checks if a given primitive kind is a numeric type.
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

// canDirectlyConvertPrimitives checks if two primitive types can be directly converted in Go.
// This is a simplified check and might need to be more comprehensive for all Go rules.
func canDirectlyConvertPrimitives(sourceKind, targetKind string) bool {
	// Identical types can always be converted (no-op)
	if sourceKind == targetKind {
		return true
	}

	// Numeric types can generally be converted to other numeric types
	if isNumericPrimitive(sourceKind) && isNumericPrimitive(targetKind) {
		return true
	}

	// string and []byte, []rune can be converted
	if (sourceKind == "string" && (targetKind == "[]byte" || targetKind == "[]rune")) ||
		((sourceKind == "[]byte" || targetKind == "[]rune") && targetKind == "string") {
		return true
	}

	// bool, string, and other non-numeric primitives usually cannot be directly converted to each other
	// unless they are identical.
	return false
}

// generatePrimitiveConversionStub generates a stub function for primitive type conversion.
func generatePrimitiveConversionStub(funcName, sourceTypeName, targetTypeName string) string {
	return fmt.Sprintf(`
// %s converts a %s to a %s.
// TODO: Implement the conversion logic for %s to %s.
func %s(from %s) %s {
	// Example: return %s(from) if direct conversion is possible and desired,
	// or implement custom logic.
	panic("not implemented") // TODO: Implement me
}
`, funcName, sourceTypeName, targetTypeName, sourceTypeName, targetTypeName, funcName, sourceTypeName, targetTypeName, targetTypeName)
}

func (g *LegacyGenerator) getConversionExpression(
	parentSource *model.TypeInfo, sourceField *model.FieldInfo,
	parentTarget *model.TypeInfo, targetField *model.FieldInfo,
	fromVar string,
) (expression string, returnsPointer bool, isFunctionCall bool) {
	sourceType := sourceField.Type
	targetType := targetField.Type
	sourceFieldExpr := fromVar
	if sourceField.Name != "" {
		sourceFieldExpr = fmt.Sprintf("%s.%s", fromVar, sourceField.Name)
	}

	// Debug: Log the conversion being attempted
	slog.Debug("getConversionExpression",
		"sourceField", sourceField.Name,
		"sourceType.Kind", sourceType.Kind,
		"sourceType.Name", sourceType.Name,
		"targetField", targetField.Name,
		"targetType.Kind", targetType.Kind,
		"targetType.Name", targetType.Name,
	)

	// Prioritize slice conversion logic
	if sourceType.Kind == model.Slice && targetType.Kind == model.Slice {
		sourceElem := g.converter.GetElementType(sourceType)
		targetElem := g.converter.GetElementType(targetType)

		if sourceElem != nil && targetElem != nil {
			if sourceElem.UniqueKey() != targetElem.UniqueKey() {
				// 🔧 关键修复：确保切片类型在 writeAliases() 之前就被添加到 requiredAliases
				g.requiredAliases[sourceType.UniqueKey()] = struct{}{}
				g.requiredAliases[targetType.UniqueKey()] = struct{}{}
				g.requiredAliases[sourceElem.UniqueKey()] = struct{}{}
				g.requiredAliases[targetElem.UniqueKey()] = struct{}{}

				// 确保别名映射中存在这些类型
				if _, exists := g.aliasMap[sourceType.UniqueKey()]; !exists {
					g.aliasMap[sourceType.UniqueKey()] = g.namer.GetAlias(sourceType, true)
				}
				if _, exists := g.aliasMap[targetType.UniqueKey()]; !exists {
					g.aliasMap[targetType.UniqueKey()] = g.namer.GetAlias(targetType, false)
				}
				if _, exists := g.aliasMap[sourceElem.UniqueKey()]; !exists {
					g.aliasMap[sourceElem.UniqueKey()] = g.namer.GetAlias(sourceElem, true)
				}
				if _, exists := g.aliasMap[targetElem.UniqueKey()]; !exists {
					g.aliasMap[targetElem.UniqueKey()] = g.namer.GetAlias(targetElem, false)
				}

				sliceConverterFuncName := g.namer.GetFunctionName(sourceType, targetType)
				g.requiredConversionFunctions[sliceConverterFuncName] = true

				slog.Debug("Slice conversion prepared",
					"sourceType", sourceType.TypeString(),
					"targetType", targetType.TypeString(),
					"funcName", sliceConverterFuncName)

				// Slice conversion function returns a slice, not a pointer
				return fmt.Sprintf("%s(%s)", sliceConverterFuncName, sourceFieldExpr), false, true
			}
		}
	}

	sourceKey := sourceType.UniqueKey()
	targetKey := targetType.UniqueKey()

	if sourceKey == targetKey {
		// Direct field access, return value is a pointer depending on whether sourceType itself is a pointer
		return sourceFieldExpr, sourceType.Kind == model.Pointer, false
	}

	// Check in the unified conversionFunctions map
	conversionKey := sourceKey + "->" + targetKey
	slog.Debug("getConversionExpression", "action", "checking conversionFunctions", "conversionKey", conversionKey)
	if funcName, ok := conversionFunctions[conversionKey]; ok {
		slog.Debug("getConversionExpression", "action", "found in conversionFunctions", "funcName", funcName)
		g.requiredConversionFunctions[funcName] = true
		if strings.Contains(funcName, "Time") {
			g.importMgr.Add("time")
		}
		if strings.Contains(funcName, "UUID") {
			g.importMgr.Add("github.com/google/uuid")
		}
		if strings.Contains(funcName, "Timestamp") {
			g.importMgr.Add("google.golang.org/protobuf/types/known/timestamppb")
		}
		// Determine if the helper function returns a pointer based on targetKey
		isPointerReturn := targetType.Kind == model.Pointer
		return fmt.Sprintf("%s(%s)", funcName, sourceFieldExpr), isPointerReturn, true // Helper function is a function call
	}

	// Check for custom function rules from config
	customFuncKey := sourceKey + "->" + targetKey
	if customFuncName, ok := g.config.CustomFunctionRules[customFuncKey]; ok {
		g.requiredConversionFunctions[customFuncName] = true // Mark custom function as required
		// Determine if the custom function returns a pointer based on targetKey
		isPointerReturn := targetType.Kind == model.Pointer
		return fmt.Sprintf("%s(%s)", customFuncName, sourceFieldExpr), isPointerReturn, true // Custom function is a function call
	}

	// Handle underlying types for named types that are not primitive
	// But avoid direct conversions from Map to its element type, as these are not valid in Go
	if sourceType.Underlying != nil && targetType.Underlying != nil && sourceType.Underlying.UniqueKey() == targetType.Underlying.UniqueKey() {
		// Skip if source is a Map and target is its element type
		if !(sourceType.Kind == model.Map && targetType.Kind == model.Primitive) {
			slog.Debug("getConversionExpression", "action", "underlying types match", "underlyingKey", sourceType.Underlying.UniqueKey())
			return fmt.Sprintf("%s(%s)", g.namer.GetTypeString(targetType), sourceFieldExpr), targetType.Kind == model.Pointer, true // Type conversion is a function call
		}
	}
	if sourceType.Underlying != nil && sourceType.Underlying.UniqueKey() == targetKey {
		// Skip if source is a Map and target is its element type
		if !(sourceType.Kind == model.Map && targetType.Kind == model.Primitive) {
			slog.Debug("getConversionExpression", "action", "source underlying matches target", "underlyingKey", sourceType.Underlying.UniqueKey())
			return fmt.Sprintf("%s(%s)", g.namer.GetTypeString(targetType), sourceFieldExpr), targetType.Kind == model.Pointer, true // Type conversion is a function call
		}
	}
	//if targetType.Underlying != nil && targetType.Underlying.UniqueKey() == sourceKey {
	//	slog.Debug("getConversionExpression", "action", "target underlying matches source", "underlyingKey", targetType.Underlying.UniqueKey())
	//	return fmt.Sprintf("%s(%s)", g.namer.GetTypeString(targetType), sourceFieldExpr), targetType.Kind == model.Pointer, true // Type conversion is a function call
	//}

	// Handle numeric primitive conversions (e.g., int to int32)
	if sourceType.Kind == model.Primitive && targetType.Kind == model.Primitive &&
		isNumericPrimitive(sourceType.Name) && isNumericPrimitive(targetType.Name) {
		return fmt.Sprintf("%s(%s)", g.namer.GetTypeString(targetType), sourceFieldExpr), false, true // Type conversion is a function call, returns value
	}

	if sourceType.Kind == model.Primitive && targetType.Kind == model.Primitive {
		if canDirectlyConvertPrimitives(sourceType.Name, targetType.Name) {
			return fmt.Sprintf("%s(%s)", g.namer.GetTypeString(targetType), sourceFieldExpr), false, true // Type conversion is a function call, returns value
		} else {
			stubFuncName := g.namer.GetPrimitiveConversionStubName(parentSource, sourceField, parentTarget, targetField)
			if _, exists := g.customStubs[stubFuncName]; !exists {
				g.customStubs[stubFuncName] = generatePrimitiveConversionStub(
					stubFuncName,
					g.namer.GetTypeString(sourceType),
					g.namer.GetTypeString(targetType),
				)
			}
			g.requiredConversionFunctions[stubFuncName] = true
			return fmt.Sprintf("%s(%s)", stubFuncName, sourceFieldExpr), false, true // Stub function is a function call, returns value
		}
	}

	// Fallback: all non-directly-convertible types should use unified naming rule
	// Check if this is a defined type conversion (struct-to-struct, etc.)
	if (sourceType.Kind == model.Struct || targetType.Kind == model.Struct) ||
		(sourceType.IsNamedType() && targetType.IsNamedType()) {
		slog.Debug("getConversionExpression", "action", "using GetFunctionName for defined type conversion")
		// This is a defined type conversion, use GetFunctionName
		funcName := g.namer.GetFunctionName(sourceType, targetType)
		g.requiredConversionFunctions[funcName] = true
		shouldReturnPointer := targetType.IsUltimatelyStruct()
		return fmt.Sprintf("%s(%s)", funcName, sourceFieldExpr), shouldReturnPointer, true
	}

	slog.Debug("getConversionExpression", "action", "generating primitive conversion stub")
	// This is a primitive-to-primitive or complex-to-primitive conversion that can't be handled directly
	// This includes cases like map[string]string -> string, string -> map[string]string, etc.
	// Use the unified naming rule: prefix+[type+fieldname]+suffix+TO+prefix+[type+fieldname]+suffix
	stubFuncName := g.namer.GetPrimitiveConversionStubName(parentSource, sourceField, parentTarget, targetField)
	slog.Debug("getConversionExpression", "stubFuncName", stubFuncName)
	if _, exists := g.customStubs[stubFuncName]; !exists {
		g.customStubs[stubFuncName] = generatePrimitiveConversionStub(
			stubFuncName,
			g.namer.GetTypeString(sourceType),
			g.namer.GetTypeString(targetType),
		)
		slog.Debug("getConversionExpression", "action", "created new stub", "stubFuncName", stubFuncName)
	}
	g.requiredConversionFunctions[stubFuncName] = true
	return fmt.Sprintf("%s(%s)", stubFuncName, sourceFieldExpr), false, true // Stub function is a function call, returns value
}

// populateAliases populates the aliasMap with generated aliases for source and target types.
func (g *LegacyGenerator) populateAliases() {
	for _, rule := range g.config.ConversionRules {
		sourceInfo := g.typeInfos[rule.SourceType]
		targetInfo := g.typeInfos[rule.TargetType]
		if sourceInfo == nil || targetInfo == nil {
			continue
		}

		// 确保基础类型有别名
		g.ensureTypeAlias(sourceInfo, true)
		g.ensureTypeAlias(targetInfo, false)
	}
}

func (g *LegacyGenerator) discoverImplicitConversionRules() {
	typesByPackage := make(map[string][]*model.TypeInfo)
	for _, info := range g.typeInfos {
		if info.IsNamedType() {
			typesByPackage[info.ImportPath] = append(typesByPackage[info.ImportPath], info)
		}
	}

	var initialRules []*config.ConversionRule

	for _, pair := range g.config.PackagePairs {
		sourceTypes := typesByPackage[pair.SourcePath]
		targetTypes := typesByPackage[pair.TargetPath]

		targetMap := make(map[string]*model.TypeInfo)
		for _, tt := range targetTypes {
			targetMap[tt.Name] = tt
		}

		for _, sourceType := range sourceTypes {
			if targetType, ok := targetMap[sourceType.Name]; ok {
				rule := &config.ConversionRule{
					SourceType: sourceType.UniqueKey(),
					TargetType: targetType.UniqueKey(),
					Direction:  config.DirectionBoth,
					FieldRules: config.FieldRuleSet{Ignore: make(map[string]struct{}), Remap: make(map[string]string)},
				}
				initialRules = append(initialRules, rule)
			}
		}
	}

	g.config.ConversionRules = append(g.config.ConversionRules, initialRules...)
}

func (g *LegacyGenerator) getPackageName() string {
	if g.config.GenerationContext.PackageName != "" {
		return g.config.GenerationContext.PackageName
	}
	return "generated"
}

func (g *LegacyGenerator) isCurrentPackage(importPath string) bool {
	return importPath == g.config.GenerationContext.PackagePath
}

// ensureTypeAlias 确保指定类型有别名，如果没有则创建
func (g *LegacyGenerator) ensureTypeAlias(typeInfo *model.TypeInfo, isSource bool) {
	if typeInfo == nil {
		return
	}

	uniqueKey := typeInfo.UniqueKey()

	// 如果别名已存在，直接返回
	if _, exists := g.aliasMap[uniqueKey]; exists {
		return
	}

	// 创建新别名
	alias := g.namer.GetAlias(typeInfo, isSource)

	// 立即存储到映射中
	g.aliasMap[uniqueKey] = alias
	g.requiredAliases[uniqueKey] = struct{}{}

	// 递归处理复合类型的元素类型
	g.ensureElementTypeAlias(typeInfo, isSource)
}

// ensureElementTypeAlias 递归确保元素类型的别名
func (g *LegacyGenerator) ensureElementTypeAlias(typeInfo *model.TypeInfo, isSource bool) {
	switch typeInfo.Kind {
	case model.Slice, model.Array, model.Pointer:
		if typeInfo.Underlying != nil {
			g.ensureTypeAlias(typeInfo.Underlying, isSource)
		}
	case model.Map:
		if typeInfo.KeyType != nil {
			g.ensureTypeAlias(typeInfo.KeyType, isSource)
		}
		if typeInfo.Underlying != nil {
			g.ensureTypeAlias(typeInfo.Underlying, isSource)
		}
	case model.Struct:
		// 为命名的结构体字段类型递归创建别名
		for _, field := range typeInfo.Fields {
			g.ensureTypeAlias(field.Type, isSource)
		}
	}
}
