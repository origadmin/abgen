package fieldgen

import (
	"bytes"
	"fmt"
	goast "go/ast"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	tmpl "text/template"

	"github.com/origadmin/abgen/internal/ast"
	"github.com/origadmin/abgen/internal/types"
)

// TemplateData 模板数据结构
type TemplateData struct {
	Src      string // 源字段引用: src.FieldName
	SrcName  string // 源字段名: FieldName
	SrcType  string // 源字段类型
	Dst      string // 目标字段引用: dst.FieldName
	DstName  string // 目标字段名: FieldName
	DstType  string // 目标字段类型
	TypeName string // 目标类型名称(不带包名)
}

// ConversionRule 定义类型转换规则
type ConversionRule struct {
	// 使用标准Go模板的转换模板
	Tmpl *tmpl.Template // Use tmpl.Template

	// 简单转换的格式字符串
	Pattern string
}

// FieldGenerator is responsible for generating field conversion logic between two struct types.
// It manages type conversion rules and resolves type information to produce the appropriate code.
type FieldGenerator struct {
	resolver ast.TypeResolver

	// conversionRules stores the registered type conversion rules.
	// The key is a string in the format "srcType:dstType".
	conversionRules map[string]ConversionRule

	// templateDir is the directory where custom conversion template files are located.
	templateDir string

	// customTypeConversionRules stores user-defined type conversion rules from directives.
	customTypeConversionRules []types.TypeConversionRule

	importMgr types.ImportManager // Change to interface // Add this
}

// RegisterTypeConversion registers a type conversion rule using a Go template string.
func (fg *FieldGenerator) RegisterTypeConversion(srcType, dstType, templateStr string) error {
	key := srcType + ":" + dstType

	// 解析模板
	t, err := tmpl.New(key).Parse(templateStr) // Use tmpl.New and rename var to 't'
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	fg.conversionRules[key] = ConversionRule{
		Tmpl: t, // Use capitalized Tmpl and 't'
	}
	return nil
}

// RegisterTypePattern registers a type conversion rule using a simple format pattern.
func (fg *FieldGenerator) RegisterTypePattern(srcType, dstType, pattern string) {
	key := srcType + ":" + dstType
	fg.conversionRules[key] = ConversionRule{
		Pattern: pattern,
	}
}

// generateConversion 生成字段转换代码
func (fg *FieldGenerator) generateConversion(srcField, dstField types.StructField, cfg *types.ConversionConfig) (string, error) {
	slog.Debug("generateConversion: 开始处理字段",
		"srcFieldName", srcField.Name, "srcFieldType", srcField.Type,
		"dstFieldName", dstField.Name, "dstFieldType", dstField.Type,
	)

	srcType := srcField.Type
	dstType := dstField.Type

	// Strip pointers before looking up the rule
	baseSrcType := strings.TrimPrefix(srcType, "*")
	baseDstType := strings.TrimPrefix(dstType, "*")
	key := baseSrcType + ":" + baseDstType
	slog.Debug("generateConversion: 规则查找键", "key", key)

	// Resolve full TypeInfo for source and destination fields
	srcFieldTypeInfo, err := fg.resolver.Resolve(srcField.Type)
	if err != nil {
		slog.Error("generateConversion: failed to resolve source field type info", "type", srcField.Type, "error", err)
		return fg.unhandledConversion(srcField, dstField)
	}
	dstFieldTypeInfo, err := fg.resolver.Resolve(dstField.Type)
	if err != nil {
		slog.Error("generateConversion: failed to resolve destination field type info", "type", dstField.Type, "error", err)
		return fg.unhandledConversion(srcField, dstField)
	}

	// Determine the actual underlying types for comparison
	// If a type is an alias, get its ElemType if it's a pointer/slice/map, otherwise its base name/ImportPath
	srcUnderlyingType := srcFieldTypeInfo.ImportPath
	if srcUnderlyingType != "" && srcUnderlyingType != "builtin" {
		srcUnderlyingType += "."
	}
	srcUnderlyingType += srcFieldTypeInfo.Name

	dstUnderlyingType := dstFieldTypeInfo.ImportPath
	if dstUnderlyingType != "" && dstUnderlyingType != "builtin" {
		dstUnderlyingType += "."
	}
	dstUnderlyingType += dstFieldTypeInfo.Name

	for _, rule := range cfg.TypeConversionRules {
		slog.Debug("generateConversion: 检查类型级自定义规则",
			"ruleSourceType", rule.SourceTypeName,
			"ruleTargetType", rule.TargetTypeName,
			"srcFieldType", srcField.Type,
			"dstFieldType", dstField.Type,
		)
		// Ensure we resolve the types for comparison, as directive types might be short names or aliases.
		resolvedRuleSource, err := fg.resolver.Resolve(rule.SourceTypeName)
		if err != nil {
			slog.Debug("generateConversion: 无法解析自定义规则源类型", "type", rule.SourceTypeName, "error", err)
			continue
		}
		resolvedRuleTarget, err := fg.resolver.Resolve(rule.TargetTypeName)
		if err != nil {
			slog.Debug("generateConversion: 无法解析自定义规则目标类型", "type", rule.TargetTypeName, "error", err)
			continue
		}

		// Resolve the actual field types for comparison
		srcFieldTypeInfo, err := fg.resolver.Resolve(srcField.Type)
		if err != nil {
			slog.Debug("generateConversion: 无法解析源字段类型", "type", srcField.Type, "error", err)
			continue
		}
		dstFieldTypeInfo, err := fg.resolver.Resolve(dstField.Type)
		if err != nil {
			slog.Debug("generateConversion: 无法解析目标字段类型", "type", dstField.Type, "error", err)
			continue
		}

		// Compare fully qualified names derived from TypeInfo
		srcMatch := (resolvedRuleSource.ImportPath == srcFieldTypeInfo.ImportPath && resolvedRuleSource.Name == srcFieldTypeInfo.Name)
		dstMatch := (resolvedRuleTarget.ImportPath == dstFieldTypeInfo.ImportPath && resolvedRuleTarget.Name == dstFieldTypeInfo.Name)

		if srcMatch && dstMatch {
			slog.Debug("generateConversion: 应用 Type-level Custom Rule (FQ TypeInfo Match)",
				"SourceRule", resolvedRuleSource.ImportPath+"."+resolvedRuleSource.Name,
				"TargetRule", resolvedRuleTarget.ImportPath+"."+resolvedRuleTarget.Name,
				"SourceField", srcFieldTypeInfo.ImportPath+"."+srcFieldTypeInfo.Name,
				"TargetField", dstFieldTypeInfo.ImportPath+"."+dstFieldTypeInfo.Name,
				"Func", rule.ConvertFunc,
			)
			return fmt.Sprintf("dst.%s = %s(src.%s)", dstField.Name, rule.ConvertFunc, srcField.Name), nil
		}
	}

	// 2. Handle remapped fields (high priority after custom rules)
	if sourcePath, exists := cfg.RemapFields[dstField.Name]; exists {
		return fg.generateRemapConversion(srcField, dstField, sourcePath, cfg)
	}

	// 3. 检查直接类型映射 (来自 RegisterTypeConversion/Pattern)
	if rule, exists := fg.conversionRules[key]; exists {
		slog.Debug("generateConversion: 命中 fg.conversionRules", "key", key)
		if rule.Tmpl != nil { // Use capitalized Tmpl
			// 准备模板数据
			data := TemplateData{
				Src:      "src." + srcField.Name,
				SrcName:  srcField.Name,
				SrcType:  srcField.Type,
				Dst:      "dst." + dstField.Name,
				DstName:  dstField.Name,
				DstType:  dstField.Type,
				TypeName: strings.TrimPrefix(dstField.Type, "*"),
			}

			// 执行模板
			var buf bytes.Buffer
			if err := rule.Tmpl.Execute(&buf, data); err != nil { // Use capitalized Tmpl
				slog.Error("generateConversion: 模板执行失败", "key", key, "error", err)
				return "", fmt.Errorf("failed to execute template: %w", err)
			}
			return buf.String(), nil

		} else if rule.Pattern != "" {
			slog.Debug("generateConversion: 应用 fg.conversionRules 模式", "key", key, "pattern", rule.Pattern)
			// 使用模式字符串生成代码
			return fmt.Sprintf(rule.Pattern,
				"dst."+dstField.Name,
				"src."+srcField.Name), nil // Fixed 'il' to 'nil'
		}
	}

	// 2. 检查切片类型
	if strings.HasPrefix(srcType, "[]") && strings.HasPrefix(dstType, "[]") {
		srcElem := strings.TrimPrefix(srcType, "[]")
		dstElem := strings.TrimPrefix(dstType, "[]")

		// 解析元素类型以获取其别名
		dstElemInfo, err := fg.resolver.Resolve(dstElem)
		if err != nil {
			slog.Warn("无法解析目标切片元素类型", "type", dstElem, "error", err)
			return fg.unhandledConversion(srcField, dstField)
		}

		// 为元素类型构建转换函数名
		funcName, err := fg.buildElementConversionFuncName(srcElem, dstElem, cfg)
		if err != nil {
			slog.Warn("无法为切片元素构建转换函数", "srcElem", srcElem, "dstElem", dstElem, "error", err)
			return fg.unhandledConversion(srcField, dstField)
		}

		// Construct the destination slice type string for make using the aliased element type.
		dstSliceType := "[]"
		if strings.HasPrefix(dstElem, "*") { // If the element type is a pointer, add '*'
			dstSliceType += "*"
		}
		dstSliceType += fg.importMgr.GetType(dstElemInfo.ImportPath, dstElemInfo.Name) // Use importMgr for alias

		// 判断源元素是否为指针，以决定循环中传递 item 还是 &item
		srcItemRef := "item"
		if !strings.HasPrefix(srcElem, "*") {
			srcItemRef = "&" + srcItemRef
		}

		// 生成 for 循环转换代码
		return fmt.Sprintf(`
if src.%s != nil {
	dst.%s = make(%s, len(src.%s))
	for i, item := range %s {
		dst.%s[i] = %s(%s)
	}
}`, srcField.Name, dstField.Name, dstSliceType, srcField.Name, srcField.Name, dstField.Name, funcName, srcItemRef), nil
	}

	// 3. Handle struct-to-struct conversions (pointer or value)
	isSrcStruct := !strings.HasPrefix(srcType, "[]") && !types.IsPrimitiveType(baseSrcType)
	isDstStruct := !strings.HasPrefix(dstType, "[]") && !types.IsPrimitiveType(baseDstType)

	if isSrcStruct && isDstStruct {
		funcName, err := fg.buildElementConversionFuncName(baseSrcType, baseDstType, cfg)
		if err == nil {
			// Conversion function found, now generate the call.
			srcArg := "src." + srcField.Name

			// The generated conversion functions always expect a pointer.
			// If the source field is a value type, take its address.
			if !strings.HasPrefix(srcType, "*") {
				srcArg = "&" + srcArg
			}

			// Only add a nil check if the source field is a pointer.
			if strings.HasPrefix(srcType, "*") {
				return fmt.Sprintf(`
if src.%s != nil {
	dst.%s = %s(%s)
}`, srcField.Name, dstField.Name, funcName, srcArg), nil
			} else {
				// If it's a value type, we can't check for nil.
				// The conversion function will receive a pointer to it.
				return fmt.Sprintf("dst.%s = %s(%s)", dstField.Name, funcName, srcArg), nil
			}
		}
	}

	// 4. 基本类型处理
	if types.IsPrimitiveType(srcType) && types.IsPrimitiveType(dstType) {
		if srcType == dstType {
			return fmt.Sprintf("dst.%s = src.%s", dstField.Name, srcField.Name), nil
		} else if types.IsNumberType(srcType) && types.IsNumberType(dstType) {
			return fmt.Sprintf("dst.%s = %s(src.%s)", dstField.Name, dstType, srcField.Name), nil
		}
	}

	// 5. 类型相同直接赋值
	if srcType == dstType {
		return fmt.Sprintf("dst.%s = src.%s", dstField.Name, srcField.Name), nil
	}

	// 6. 默认情况
	return fg.unhandledConversion(srcField, dstField)
}

func (fg *FieldGenerator) unhandledConversion(srcField, dstField types.StructField) (string, error) {
	slog.Warn("unhandled type conversion", "srcType", srcField.Type, "dstType", dstField.Type)
	return fmt.Sprintf("// WARNING: unhandled type conversion\n // dst.%s = src.%s (%s -> %s)",
		dstField.Name, srcField.Name, srcField.Type, dstField.Type), nil
}

// buildElementConversionFuncName builds the full conversion function name for nested elements (pointers/slices).
func (fg *FieldGenerator) buildElementConversionFuncName(srcElem, dstElem string, cfg *types.ConversionConfig) (string, error) {
	slog.Debug("buildElementConversionFuncName: 开始处理", "srcElem", srcElem, "dstElem", dstElem)
	// 完整类型路径
	fullSrcType := strings.TrimPrefix(srcElem, "*")
	fullDstType := strings.TrimPrefix(dstElem, "*")

	srcInfo, err := fg.resolver.Resolve(fullSrcType)
	if err != nil {
		slog.Error("buildElementConversionFuncName: 无法解析源元素类型", "type", fullSrcType, "error", err)
		return "", fmt.Errorf("无法解析源元素类型 %s: %w", fullSrcType, err)
	}
	dstInfo, err := fg.resolver.Resolve(fullDstType)
	if err != nil {
		slog.Error("buildElementConversionFuncName: 无法解析目标元素类型", "type", fullDstType, "error", err)
		return "", fmt.Errorf("无法解析目标元素类型 %s: %w", fullDstType, err)
	}

	// Directly apply suffixes from cfg
	srcAlias := cfg.Source.Prefix + srcInfo.Name + cfg.Source.Suffix
	dstAlias := cfg.Target.Prefix + dstInfo.Name + cfg.Target.Suffix

	slog.Debug("buildElementConversionFuncName: 生成函数名",
		"fullSrcType", fullSrcType, "srcInfo.Name", srcInfo.Name, "srcAlias", srcAlias,
		"fullDstType", fullDstType, "dstInfo.Name", dstInfo.Name, "dstAlias", dstAlias,
	)

	return fmt.Sprintf("Convert%sTo%s", srcAlias, dstAlias), nil
}

// UpdateTypeMap 更新类型映射表
func (fg *FieldGenerator) UpdateTypeMap() {
	// Register base types, not pointer types.
	fg.RegisterTypePattern("time.Time", "google.golang.org/protobuf/types/known/timestamppb.Timestamp",
		"%s = timestamppb.New(%s)")

	fg.RegisterTypePattern("google.golang.org/protobuf/types/known/timestamppb.Timestamp", "time.Time",
		"%s = %s.AsTime()")

	// 数字类型处理
	fg.RegisterTypePattern("uint64", "int64",
		"%s = int64(%s)")

	fg.RegisterTypePattern("uint32", "int32",
		"%s = int32(%s)")

	// UUID处理
	fg.RegisterTypePattern("github.com/google/uuid.UUID", "string",
		"%s = %s.String()")

	// 特殊类型处理 - 使用Go模板
	err := fg.RegisterTypeConversion("map[string]interface{}}", "google.golang.org/protobuf/types/known/structpb.Struct", `
{{if .Src}}
var err error
{{.Dst}}, err = structpb.NewStruct({{.Src}})
if err != nil {
	slog.Error("failed to convert field", "field", "{{.SrcName}}", "error", err)
}
{{end}}
`)
	if err != nil {
		slog.Error("failed to register template", "error", err)
	}

	// 使用条件逻辑的模板示例
	err = fg.RegisterTypeConversion("[]string", "[]int", `
{{if .Src}}
{{.Dst}} = make([]int, 0, len({{.Src}}))
for _, v := range {{.Src}} {
	if num, err := strconv.Atoi(v); err == nil {
		{{.Dst}} = append({{.Dst}}, num)
	}
}
{{end}}
`)
	if err != nil {
		slog.Error("failed to register template", "error", err)
	}
}

// generateRemapConversion handles generating code for remapped fields.
func (fg *FieldGenerator) generateRemapConversion(srcField, dstField types.StructField, sourcePath string, cfg *types.ConversionConfig) (string, error) {
	slog.Debug("generateRemapConversion: handling remap", "dstField", dstField.Name, "sourcePath", sourcePath)

	// First, resolve the TypeInfo of the root source object (e.g., *ent.User for "src")
	// The srcField.Type will be "*ent.User" if this is called for a root field,
	// but it's passed as a specific field in the loop in GenerateFields.
	// The 'sourceType' parameter to GenerateFields is the overall Source struct type.
	// We need the TypeInfo for that overall Source struct.

	// Assuming sourcePath starts from the root of the 'src' object passed to the conversion function
	// So, the baseTypeInfo for resolvePathType should be the TypeInfo of cfg.Source.Type.
	srcRootTypeInfo, err := fg.resolver.Resolve(cfg.Source.Type)
	if err != nil {
		return "", fmt.Errorf("generateRemapConversion: failed to resolve source root type %s: %w", cfg.Source.Type, err)
	}

	// Resolve the actual type of the sourcePath (e.g., for "Edges.Roles", get type of src.Edges.Roles)
	resolvedSourcePathType, err := fg.resolvePathType(srcRootTypeInfo, sourcePath)
	if err != nil {
		return "", fmt.Errorf("generateRemapConversion: failed to resolve type of source path %s: %w", sourcePath, err)
	}

	slog.Debug("generateRemapConversion: resolved source path type",
		"sourcePath", sourcePath,
		"resolvedType", resolvedSourcePathType.Name, // Use .Name
		"isSlice", strings.HasPrefix(resolvedSourcePathType.Name, "[]"), // Use .Name
		"isPointer", strings.HasPrefix(resolvedSourcePathType.Name, "*"), // Use .Name
	)

	// Construct the full source access string (e.g., "src.Edges.Roles" or "src.Edges.Roles.ID")
	fullSourceAccess := fmt.Sprintf("src.%s", sourcePath)

	// If the resolved source path is a slice, and the destination is not a slice, then it's a projection (e.g., RolesIDs)
	if strings.HasPrefix(resolvedSourcePathType.Name, "[]") { // Use .Name for slice check
		slog.Info("generateRemapConversion: detected slice in source path, checking for projection", "sourcePath", sourcePath)

		// If the destination field is a slice, and the last part of sourcePath refers to a struct type,
		// it's likely a slice-to-slice conversion (e.g. Roles -> Roles)
		// We need to compare resolvedSourcePathType.ElemType with dstField.ElemType

		// If the destination field is NOT a slice, and the last part of sourcePath is "ID" or "IDs", it's a slice projection for IDs
		// This needs more granular checks

		// For RoleIDs:Edges.Roles.ID
		// sourcePath = "Edges.Roles.ID"
		// resolvedSourcePathType = TypeInfo for 'int64' (if ID is int64)
		// But the original source is src.Edges.Roles which is a slice.
		// This means sourcePath itself might already represent the element to project.

		// Need to rethink how sourcePath is defined for slice projections.
		// If `remap="RoleIDs:Edges.Roles.ID"`
		// It means `dst.RoleIDs` (int64) comes from `src.Edges.Roles` ( []*ent.Role).
		// We need to iterate over `src.Edges.Roles` and extract `ID` from each.

		// Temporarily return a specific TODO for slice projection that needs a helper function.
		return fmt.Sprintf("// TODO: Implement slice projection helper for %s from %s", dstField.Name, fullSourceAccess), nil
	} else { // Source path is a single struct or primitive, not a slice
		slog.Info("generateRemapConversion: detected simple nested field copy", "sourcePath", sourcePath)
		// This is a simple nested struct/field copy.
		// Need to call conversion function if types are different, or direct assign if compatible.

		// If types are different, need to find/generate a conversion function.
		if resolvedSourcePathType.Name != dstField.Type { // Use .Name
			funcName, err := fg.buildElementConversionFuncName(resolvedSourcePathType.Name, dstField.Type, cfg) // Use .Name
			if err != nil {
				return "", fmt.Errorf("failed to build conversion func name for remapped field: %w", err)
			}

			// Determine if source needs address taken
			srcArg := fullSourceAccess
			if !strings.HasPrefix(resolvedSourcePathType.Name, "*") { // Use .Name for pointer check
				srcArg = "&" + srcArg
			}

			return fmt.Sprintf("dst.%s = %s(%s)", dstField.Name, funcName, srcArg), nil
		} else {
			// Direct assignment if types are compatible (or same)
			return fmt.Sprintf("dst.%s = %s", dstField.Name, fullSourceAccess), nil
		}
	}
}

// GenerateFields generates a list of field conversion snippets for a given source and target type.
// It uses the registered rules and built-in logic to determine the conversion code for each matching field.
func (fg *FieldGenerator) GenerateFields(sourceType, targetType string, cfg *types.ConversionConfig) []types.FieldConversion {
	var fields []types.FieldConversion

	// Register custom type conversion rules from directives
	for _, rule := range fg.customTypeConversionRules {
		// The pattern for custom functions will be "dst.Field = ConvertFunc(src.Field)"
		// The existing pattern usage in generateConversion is: fmt.Sprintf(rule.Pattern, "dst."+dstField.Name, "src."+srcField.Name)
		// So, the pattern should be "%s = ConvertFunc(%s)"
		pattern := fmt.Sprintf("%%s = %s(%%s)", rule.ConvertFunc)
		fg.RegisterTypePattern(rule.SourceTypeName, rule.TargetTypeName, pattern)
		slog.Debug("Registered custom type conversion rule", "srcType", rule.SourceTypeName, "dstType", rule.TargetTypeName, "func", rule.ConvertFunc)
	}

	// 获取源类型和目标类型信息
	slog.Info("resolving source type", "type", sourceType)
	srcInfo, err := fg.resolver.Resolve(sourceType)
	if err != nil {
		slog.Error("failed to resolve source type", "type", sourceType, "error", err)
		return fields
	}

	slog.Info("resolving target type", "type", targetType)
	dstInfo, err := fg.resolver.Resolve(targetType)
	if err != nil {
		slog.Error("failed to resolve target type", "type", targetType, "error", err)
		return fields
	}

	// 创建目标字段映射
	dstFieldMap := make(map[string]types.StructField)
	for _, f := range dstInfo.Fields {
		dstFieldMap[strings.ToLower(f.Name)] = f
	}

	// 生成字段转换
	for _, srcField := range srcInfo.Fields {
		if !goast.IsExported(srcField.Name) {
			continue
		}
		if cfg.IgnoreFields[srcField.Name] {
			fields = append(fields, types.FieldConversion{
				Name:         srcField.Name,
				Ignore:       true,
				IgnoreReason: "configured to ignore",
			})
			continue
		}

		// 查找匹配的目标字段
		dstField, exists := dstFieldMap[strings.ToLower(srcField.Name)]
		if !exists {
			continue
		}

		// 生成转换代码
		conversion, err := fg.generateConversion(srcField, dstField, cfg)
		if err != nil {
			slog.Error("failed to generate conversion code", "field", srcField.Name, "error", err)
			continue
		}
		fieldConv := types.FieldConversion{
			Name:       srcField.Name,
			Ignore:     false,
			Conversion: conversion,
		}
		fields = append(fields, fieldConv)
	}

	return fields
}

// New creates and initializes a new FieldGenerator instance.
func New(customRules []types.TypeConversionRule, importMgr types.ImportManager) *FieldGenerator {
	fg := &FieldGenerator{
		conversionRules:           make(map[string]ConversionRule),
		customTypeConversionRules: customRules,
		importMgr:                 importMgr, // Assign the import manager
	}
	fg.UpdateTypeMap()
	return fg
}

// SetTemplateDir sets the directory from which to load custom conversion templates.
func (fg *FieldGenerator) SetTemplateDir(dir string) {
	fg.templateDir = dir
}

// SetCustomTypeConversionRules sets the custom type conversion rules from directives.
func (fg *FieldGenerator) SetCustomTypeConversionRules(rules []types.TypeConversionRule) { // Fixed type
	fg.customTypeConversionRules = rules
}

// LoadTemplatesFromDir loads and parses all template files (*.tpl) from the configured template directory.
// It registers each discovered template as a type conversion rule.
func (fg *FieldGenerator) LoadTemplatesFromDir() error {
	if fg.templateDir == "" {
		return nil // 没有设置目录，跳过加载
	}

	files, err := os.ReadDir(fg.templateDir)
	if err != nil {
		return fmt.Errorf("failed to read template directory: %w", err)
	}

	// 创建主模板
	masterTmpl := tmpl.New("master") // Use tmpl.New

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".tpl") {
			continue
		}

		// 读取模板内容
		content, err := os.ReadFile(filepath.Join(fg.templateDir, file.Name()))
		if err != nil {
			slog.Error("failed to read template file", "file", file.Name(), "error", err)
			continue
		}

		// 解析模板
		_, err = masterTmpl.Parse(string(content))
		if err != nil {
			slog.Error("failed to parse template content", "file", file.Name(), "error", err)
			continue
		}
	}

	// 查找并注册所有定义的转换模板
	for _, t := range masterTmpl.Templates() { // Changed tmpl to t to avoid variable shadowing
		name := t.Name() // Use t.Name()
		if !strings.Contains(name, ":") || name == "master" {
			continue // 跳过非转换模板
		}

		parts := strings.Split(name, ":")
		if len(parts) != 2 {
			continue
		}

		srcType := parts[0]
		dstType := parts[1]

		// 注册模板
		fg.conversionRules[srcType+":"+dstType] = ConversionRule{
			Tmpl: t, // Use capitalized Tmpl and 't'
		}
		slog.Info("registered template from file", "source_type", srcType, "target_type", dstType)
	}

	return nil
}

// SetImportManager sets the import manager for the field generator.
func (fg *FieldGenerator) SetImportManager(im types.ImportManager) {
	fg.importMgr = im
}

// SetResolver sets the type resolver for the generator.
// The resolver is used to inspect and get information about Go source types.
func (fg *FieldGenerator) SetResolver(resolver ast.TypeResolver) {
	fg.resolver = resolver
}

// resolvePathType resolves the TypeInfo for a nested field path starting from a base TypeInfo.
// Example: baseTypeInfo for "User", fieldPath "Edges.Roles"
func (fg *FieldGenerator) resolvePathType(baseTypeInfo types.TypeInfo, fieldPath string) (types.TypeInfo, error) {
	currentTypeInfo := baseTypeInfo
	parts := strings.Split(fieldPath, ".")

	for _, part := range parts {
		// If currentTypeInfo is a pointer, resolve its element type.
		if strings.HasPrefix(currentTypeInfo.Name, "*") { // Corrected: Use .Name for pointer check
			resolvedPtrType, err := fg.resolver.Resolve(strings.TrimPrefix(currentTypeInfo.Name, "*")) // Corrected: Use .Name
			if err != nil {
				return types.TypeInfo{}, fmt.Errorf("failed to resolve pointer type %s: %w", currentTypeInfo.Name, err)
			}
			currentTypeInfo = resolvedPtrType
		}

		// If currentTypeInfo is a slice, resolve its element type.
		if strings.HasPrefix(currentTypeInfo.Name, "[]") { // Corrected: Use .Name for slice check
			resolvedSliceType, err := fg.resolver.Resolve(strings.TrimPrefix(currentTypeInfo.Name, "[]")) // Corrected: Use .Name
			if err != nil {
				return types.TypeInfo{}, fmt.Errorf("failed to resolve slice element type %s: %w", currentTypeInfo.Name, err)
			}
			currentTypeInfo = resolvedSliceType
		}

		if types.IsPrimitiveType(currentTypeInfo.Name) && len(parts) > 1 {
			return types.TypeInfo{}, fmt.Errorf("cannot access field %s on primitive type %s", part, currentTypeInfo.Name)
		}

		found := false
		// The resolvedStructType might be a pointer or slice itself, so we need to get its base type
		baseStructTypeName := strings.TrimPrefix(strings.TrimPrefix(currentTypeInfo.Name, "*"), "[]") // Corrected: Use .Name
		resolvedStructType, err := fg.resolver.Resolve(baseStructTypeName)
		if err != nil {
			return types.TypeInfo{}, fmt.Errorf("failed to resolve struct info for type %s: %w", baseStructTypeName, err)
		}

		for _, field := range resolvedStructType.Fields {
			if field.Name == part {
				fieldTypeInfo, err := fg.resolver.Resolve(field.Type)
				if err != nil {
					return types.TypeInfo{}, fmt.Errorf("failed to resolve type of field %s.%s: %w", currentTypeInfo.Name, field.Name, err)
				}
				currentTypeInfo = fieldTypeInfo
				found = true
				break
			}
		}

		if !found {
			return types.TypeInfo{}, fmt.Errorf("field %s not found in type %s.%s", part, currentTypeInfo.ImportPath, currentTypeInfo.Name)
		}
	}
	return currentTypeInfo, nil
}
