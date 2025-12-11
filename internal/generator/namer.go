// Package generator provides naming utilities for generated code.
package generator

import (
	"strings"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

// Namer handles naming conventions for generated types and functions.
type Namer struct {
	config *config.Config
}

// NewNamer creates a new Namer with the given configuration.
func NewNamer(config *config.Config) *Namer {
	return &Namer{
		config: config,
	}
}

// GetTypeName returns the type name according to naming rules.
func (n *Namer) GetTypeName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}
	
	baseName := info.Name
	
	// Check if this is a source package
	sourcePrefix, sourceSuffix := n.getSourceNamingRules(info.ImportPath)
	if sourcePrefix != "" || sourceSuffix != "" {
		// This is a source type, apply source naming rules
		return sourcePrefix + baseName + sourceSuffix
	}
	
	// Check if this is a target package
	targetPrefix, targetSuffix := n.getTargetNamingRules(info.ImportPath)
	if targetPrefix != "" || targetSuffix != "" {
		// This is a target type, apply target naming rules
		return targetPrefix + baseName + targetSuffix
	}
	
	// No specific naming rules, use original name
	return baseName
}

// GetFunctionName returns the function name for converting between two types.
func (n *Namer) GetFunctionName(sourceType, targetType *model.TypeInfo) string {
	// 根据文档规范：Convert + [SourceTypeName] + To + [TargetTypeName]
	sourceName := n.getTypeDisplayName(sourceType)
	targetName := n.getTypeDisplayName(targetType)
	
	// 对于基本类型，直接使用类型名
	if sourceType.Kind == model.Primitive {
		sourceName = strings.Title(sourceType.Name)
	}
	if targetType.Kind == model.Primitive {
		targetName = strings.Title(targetType.Name)
	}
	
	return "Convert" + sourceName + "To" + targetName
}

// getSourceNamingRules returns the naming rules for the source package.
func (n *Namer) getSourceNamingRules(pkgPath string) (string, string) {
	// Check if this is the source package in any pair
	for _, pair := range n.config.PackagePairs {
		if pkgPath == pair.SourcePath {
			return n.config.NamingRules.SourcePrefix, n.config.NamingRules.SourceSuffix
		}
		if pkgPath == pair.TargetPath {
			return n.config.NamingRules.TargetPrefix, n.config.NamingRules.TargetSuffix
		}
	}
	return "", ""
}

// getTargetNamingRules returns the naming rules for the target package.
func (n *Namer) getTargetNamingRules(pkgPath string) (string, string) {
	// Check if this is the target package in any pair
	for _, pair := range n.config.PackagePairs {
		if pkgPath == pair.TargetPath {
			return n.config.NamingRules.TargetPrefix, n.config.NamingRules.TargetSuffix
		}
	}
	return "", ""
}

// getSimpleTypeName returns the simple type name without package prefixes.
func (n *Namer) getSimpleTypeName(t *model.TypeInfo) string {
	if t == nil {
		return ""
	}
	
	// Use base name
	name := t.Name
	if t.Kind == model.Pointer {
		name = strings.TrimPrefix(name, "*")
	}
	if t.Kind == model.Slice {
		name = strings.TrimPrefix(name, "[]")
	}
	return name
}

// getTypeDisplayName returns the display name for a type, applying naming rules if applicable.
// 根据规范文档实现类型命名规则
func (n *Namer) getTypeDisplayName(t *model.TypeInfo) string {
	if t == nil {
		return ""
	}
	
	name := n.getSimpleTypeName(t)
	
	// 根据规范：如果源类型和目标类型都没有定义任何Prefix和Suffix，自动添加Source/Target后缀
	if t.ImportPath != "" {
		sourcePrefix, sourceSuffix := n.getSourceNamingRules(t.ImportPath)
		targetPrefix, targetSuffix := n.getTargetNamingRules(t.ImportPath)
		
		// 检查是否为源包
		if sourcePrefix != "" || sourceSuffix != "" {
			name = sourcePrefix + name + sourceSuffix
		} else if targetPrefix != "" || targetSuffix != "" {
			// 检查是否为目标包
			name = targetPrefix + name + targetSuffix
		} else {
			// 检查是否需要自动添加Source/Target后缀
			// 只有在源类型和目标类型都没有定义任何Prefix和Suffix时才自动添加
			needsSourceSuffix := n.packageNeedsSourceSuffix(t.ImportPath)
			needsTargetSuffix := n.packageNeedsTargetSuffix(t.ImportPath)
			
			if needsSourceSuffix && !strings.HasSuffix(name, "Source") {
				name = name + "Source"
			} else if needsTargetSuffix && !strings.HasSuffix(name, "Target") {
				name = name + "Target"
			}
		}
	}
	
	return name
}

// 检查包是否需要Source后缀（简化实现）
func (n *Namer) packageNeedsSourceSuffix(pkgPath string) bool {
	// 检查是否为任何包对中的源包，且没有定义source命名规则
	for _, pair := range n.config.PackagePairs {
		if pkgPath == pair.SourcePath {
			sourcePrefix, sourceSuffix := n.getSourceNamingRules(pkgPath)
			return sourcePrefix == "" && sourceSuffix == ""
		}
	}
	return false
}

// 检查包是否需要Target后缀（简化实现）
func (n *Namer) packageNeedsTargetSuffix(pkgPath string) bool {
	// 检查是否为任何包对中的目标包，且没有定义target命名规则
	for _, pair := range n.config.PackagePairs {
		if pkgPath == pair.TargetPath {
			targetPrefix, targetSuffix := n.getTargetNamingRules(pkgPath)
			return targetPrefix == "" && targetSuffix == ""
		}
	}
	return false
}

// GetAliasName returns the alias name for a given model.TypeInfo, applying target naming rules.
func (n *Namer) GetAliasName(t *model.TypeInfo) string {
	if t == nil {
		return ""
	}
	baseName := t.Name // Use the base name from the model.TypeInfo

	// Apply target package naming rules from the Config
	// This assumes the alias is being generated for a 'target' type.
	result := baseName
	if n.config.NamingRules.TargetPrefix != "" {
		result = n.config.NamingRules.TargetPrefix + result
	}
	if n.config.NamingRules.TargetSuffix != "" {
		result = result + n.config.NamingRules.TargetSuffix
	}
	return result
}
