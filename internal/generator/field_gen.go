package generator

import (
	"bytes"
	"fmt"
	goast "go/ast"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"

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
	tmpl *template.Template

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
}

// RegisterTypeConversion registers a type conversion rule using a Go template string.
func (fg *FieldGenerator) RegisterTypeConversion(srcType, dstType, templateStr string) error {
	key := srcType + ":" + dstType

	// 解析模板
	tmpl, err := template.New(key).Parse(templateStr)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	fg.conversionRules[key] = ConversionRule{
		tmpl: tmpl,
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
	srcType := srcField.Type
	dstType := dstField.Type

	// **FIX**: Strip pointers before looking up the rule
	baseSrcType := strings.TrimPrefix(srcType, "*")
	baseDstType := strings.TrimPrefix(dstType, "*")
	key := baseSrcType + ":" + baseDstType

	// 1. 检查直接类型映射
	if rule, exists := fg.conversionRules[key]; exists {
		if rule.tmpl != nil {
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
			if err := rule.tmpl.Execute(&buf, data); err != nil {
				return "", fmt.Errorf("failed to execute template: %w", err)
			}
			return buf.String(), nil

		} else if rule.Pattern != "" {
			// 使用模式字符串生成代码
			return fmt.Sprintf(rule.Pattern,
				"dst."+dstField.Name,
				"src."+srcField.Name), nil
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

		// 使用目标类型的别名来创建切片
		dstElemAlias := createTypeAlias(dstElemInfo.Name, dstElemInfo.PkgName, "", "", cfg.GeneratorPkgPath)
		dstSliceType := fmt.Sprintf("[]*%s", dstElemAlias)
		if !strings.HasPrefix(dstElem, "*") { // 如果目标元素类型不是指针，则切片类型也不是指针切片
			dstSliceType = fmt.Sprintf("[]%s", dstElemAlias)
		}

		// 判断源元素是否为指针，以决定循环中传递 item 还是 &item
		srcItemRef := "item"
		if !strings.HasPrefix(srcElem, "*") {
			srcItemRef = "&item"
		}

		// 生成 for 循环转换代码
		return fmt.Sprintf(`
if src.%s != nil {
	dst.%s = make(%s, len(src.%s))
	for i, item := range src.%s {
		dst.%s[i] = %s(%s)
	}
}`, srcField.Name, dstField.Name, dstSliceType, srcField.Name, srcField.Name, dstField.Name, funcName, srcItemRef), nil
	}

	// 3. 检查指针 to 结构体
	if strings.HasPrefix(srcType, "*") && strings.HasPrefix(dstType, "*") {
		srcElem := strings.TrimPrefix(srcType, "*")
		dstElem := strings.TrimPrefix(dstType, "*")

		// 为元素类型构建转换函数名
		funcName, err := fg.buildElementConversionFuncName(srcElem, dstElem, cfg)
		if err != nil {
			slog.Warn("无法为指针元素构建转换函数", "srcElem", srcElem, "dstElem", dstElem, "error", err)
			return fg.unhandledConversion(srcField, dstField)
		} else {
			return fmt.Sprintf(`
if src.%s != nil {
	dst.%s = %s(src.%s)
}`, srcField.Name, dstField.Name, funcName, srcField.Name), nil
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
	// 完整类型路径
	fullSrcType := strings.TrimPrefix(srcElem, "*")
	fullDstType := strings.TrimPrefix(dstElem, "*")

	srcInfo, err := fg.resolver.Resolve(fullSrcType)
	if err != nil {
		return "", fmt.Errorf("无法解析源元素类型 %s: %w", fullSrcType, err)
	}
	dstInfo, err := fg.resolver.Resolve(fullDstType)
	if err != nil {
		return "", fmt.Errorf("无法解析目标元素类型 %s: %w", fullDstType, err)
	}

	// **FIX**: Use the unified createTypeAlias function
	srcAlias := createTypeAlias(srcInfo.Name, srcInfo.PkgName, "", "", cfg.GeneratorPkgPath)
	dstAlias := createTypeAlias(dstInfo.Name, dstInfo.PkgName, "", "", cfg.GeneratorPkgPath)

	return fmt.Sprintf("Convert%sTo%s", srcAlias, dstAlias), nil
}

// **FIX**: Unified createTypeAlias function, copied from core/generator.go
func createTypeAlias(typeName, pkgName, prefix, suffix, generatorPkgPath string) string {
	slog.Debug("createTypeAlias 输入", "原始typeName", typeName, "pkgName", pkgName, "prefix", prefix, "suffix", suffix)

	// 首先从完整路径中提取类型名
	if strings.Contains(typeName, "/") {
		parts := strings.Split(typeName, "/")
		typeName = parts[len(parts)-1]
	}

	// 如果类型名中仍包含包选择器（例如 "types.User"），则将其拆分
	if dotIndex := strings.LastIndex(typeName, "."); dotIndex != -1 {
		// 如果 pkgName 为空，则从类型名中提取它
		if pkgName == "" {
			pkgName = typeName[:dotIndex]
		}
		// 更新 typeName 为不带包选择器的纯类型名
		typeName = typeName[dotIndex+1:]
	}

	// 如果有自定义前缀或后缀，优先使用
	if prefix != "" || suffix != "" {
		result := prefix + typeName + suffix
		slog.Debug("createTypeAlias 输出 (前缀/后缀)", "result", result)
		return result
	}

	// 根据包名创建默认别名
	var result string
	switch pkgName {
	case "ent":
		result = typeName + "Ent"
	case "types":
		// 这是一个基于约定的简单检查
		if strings.HasSuffix(generatorPkgPath, "dto") { // 假设在 dto 包中，'types' 通常指向 protobuf
			result = typeName + "PB"
		} else {
			result = typeName + "Types"
		}
	case "typespb": // 直接处理 'typespb' 别名
		result = typeName + "PB"
	case "dto":
		result = typeName + "DTO"
	default:
		if pkgName != "" {
			// 使用包名的首字母大写作为后缀
			result = typeName + strings.Title(pkgName)
		} else {
			// 如果没有包名，则直接使用类型名
			result = typeName
		}
	}

	slog.Debug("createTypeAlias 输出", "result", result)
	return result
}


// UpdateTypeMap 更新类型映射表
func (fg *FieldGenerator) UpdateTypeMap() {
	// **FIX**: Register base types, not pointer types.
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
	err := fg.RegisterTypeConversion("map[string]interface{}", "google.golang.org/protobuf/types/known/structpb.Struct", `
{{if .Src}}
var err error
{{.Dst}}, err = structpb.NewStruct({{.Src}})
if err != nil {
	slog.Error("failed to convert field", "field", "{{.SrcName}}", "error", err)
}
{{end}}`)
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
{{end}}`)
	if err != nil {
		slog.Error("failed to register template", "error", err)
	}
}

// GenerateFields generates a list of field conversion snippets for a given source and target type.
// It uses the registered rules and built-in logic to determine the conversion code for each matching field.
func (fg *FieldGenerator) GenerateFields(sourceType, targetType string, cfg *types.ConversionConfig) []types.FieldConversion {
	var fields []types.FieldConversion

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

	cfg.SrcPackage = srcInfo.ImportPath
	cfg.DstPackage = dstInfo.ImportPath

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

// NewFieldGenerator creates and initializes a new FieldGenerator instance.
func NewFieldGenerator() *FieldGenerator {
	fg := &FieldGenerator{
		conversionRules: make(map[string]ConversionRule),
	}
	fg.UpdateTypeMap()
	return fg
}

// SetTemplateDir sets the directory from which to load custom conversion templates.
func (fg *FieldGenerator) SetTemplateDir(dir string) {
	fg.templateDir = dir
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
	masterTmpl := template.New("master")

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
	for _, tmpl := range masterTmpl.Templates() {
		name := tmpl.Name()
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
			tmpl: tmpl,
		}
		slog.Info("registered template from file", "source_type", srcType, "target_type", dstType)
	}

	return nil
}

// SetResolver sets the type resolver for the generator.
// The resolver is used to inspect and get information about Go source types.
func (fg *FieldGenerator) SetResolver(resolver ast.TypeResolver) {
	fg.resolver = resolver
}
