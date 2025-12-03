package generator

import (
	"bytes"
	"fmt"
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

// 内置处理器集合
var builtinHandlers = map[string]func(srcField, dstField types.StructField) (string, error){
	// 指针处理逻辑
	"pointer:pointer": func(srcField, dstField types.StructField) (string, error) {
		srcElem := strings.TrimPrefix(srcField.Type, "*")
		dstElem := strings.TrimPrefix(dstField.Type, "*")
		return fmt.Sprintf(`
if src.%s != nil {
	dst.%s = Convert%sTo%s(src.%s)
}`, srcField.Name, dstField.Name, srcElem, dstElem, srcField.Name), nil
	},

	// 相同类型切片处理
	"slice:slice:same": func(srcField, dstField types.StructField) (string, error) {
		dstElem := strings.TrimPrefix(dstField.Type, "[]")
		return fmt.Sprintf(`
if src.%s != nil {
	dst.%s = make([]%s, len(src.%s))
	copy(dst.%s, src.%s)
}`, srcField.Name, dstField.Name, dstElem, srcField.Name, dstField.Name, srcField.Name), nil
	},

	// 不同类型切片处理
	"slice:slice:diff": func(srcField, dstField types.StructField) (string, error) {
		srcElem := strings.TrimPrefix(srcField.Type, "[]")
		dstElem := strings.TrimPrefix(dstField.Type, "[]")
		return fmt.Sprintf(`
if src.%s != nil {
	dst.%s = make([]%s, 0, len(src.%s))
	for _, item := range src.%s {
		dst.%s = append(dst.%s, Convert%sTo%s(&item))
	}
}`, srcField.Name, dstField.Name, dstElem, srcField.Name, srcField.Name, dstField.Name, dstField.Name, srcElem, dstElem), nil
	},
}

// generateConversion 生成字段转换代码
func (fg *FieldGenerator) generateConversion(srcField, dstField types.StructField) (string, error) {
	srcType := srcField.Type
	dstType := dstField.Type

	// 1. 检查直接类型映射
	key := srcType + ":" + dstType
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

	// 2. 检查内置类型处理器

	// 检查指针类型
	if strings.HasPrefix(srcType, "*") && strings.HasPrefix(dstType, "*") {
		if handler, ok := builtinHandlers["pointer:pointer"]; ok {
			return handler(srcField, dstField)
		}
	}

	// 检查切片类型
	if strings.HasPrefix(srcType, "[]") && strings.HasPrefix(dstType, "[]") {
		srcElem := strings.TrimPrefix(srcType, "[]")
		dstElem := strings.TrimPrefix(dstType, "[]")

		if srcElem == dstElem {
			if handler, ok := builtinHandlers["slice:slice:same"]; ok {
				return handler(srcField, dstField)
			}
		} else {
			if handler, ok := builtinHandlers["slice:slice:diff"]; ok {
				return handler(srcField, dstField)
			}
		}
	}

	// 检查特定类型组合
	if handler, ok := builtinHandlers[key]; ok {
		return handler(srcField, dstField)
	}

	// 3. 基本类型处理
	if types.IsPrimitiveType(srcType) && types.IsPrimitiveType(dstType) {
		if srcType == dstType {
			return fmt.Sprintf("dst.%s = src.%s", dstField.Name, srcField.Name), nil
		} else if types.IsNumberType(srcType) && types.IsNumberType(dstType) {
			return fmt.Sprintf("dst.%s = %s(src.%s)", dstField.Name, dstType, srcField.Name), nil
		}
	}

	// 4. 类型相同直接赋值
	if srcType == dstType {
		return fmt.Sprintf("dst.%s = src.%s", dstField.Name, srcField.Name), nil
	}

	// 5. 默认情况
	slog.Warn("unhandled type conversion", "srcType", srcType, "dstType", dstType)
	return fmt.Sprintf("// WARNING: unhandled type conversion\n // dst.%s = src.%s (%s -> %s)",
		dstField.Name, srcField.Name, srcType, dstType), nil
}

// UpdateTypeMap 更新类型映射表
func (fg *FieldGenerator) UpdateTypeMap() {
	// 注册基础类型映射 - 使用模式而非函数
	fg.RegisterTypePattern("time.Time", "*timestamppb.Timestamp",
		"%s = timestamppb.New(%s)")

	fg.RegisterTypePattern("*timestamppb.Timestamp", "time.Time",
		"%s = %s.AsTime()")

	// 数字类型处理
	fg.RegisterTypePattern("uint64", "int64",
		"%s = int64(%s)")

	fg.RegisterTypePattern("uint32", "int32",
		"%s = int32(%s)")

	// UUID处理
	fg.RegisterTypePattern("uuid.UUID", "string",
		"%s = %s.String()")

	// 特殊类型处理 - 使用Go模板
	err := fg.RegisterTypeConversion("map[string]interface{}", "*structpb.Struct", `
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
		conversion, err := fg.generateConversion(srcField, dstField)
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
