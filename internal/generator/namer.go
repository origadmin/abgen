// Package generator provides naming utilities for generated code.
package generator

import (
	"strings"

	"github.com/origadmin/abgen/internal/analyzer"
	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

// Namer handles naming conventions for generated types and functions.
type Namer struct {
	ruleSet *config.RuleSet
}

// NewNamer creates a new Namer with the given rule set.
func NewNamer(ruleSet *config.RuleSet) *Namer {
	return &Namer{
		ruleSet: ruleSet,
	}
}

// GetTypeName returns the type name according to naming rules.
func (n *Namer) GetTypeName(info *analyzer.TypeInfo) string {
	if info == nil {
		return ""
	}
	
	baseName := info.Name
	
	// Apply source package naming rules
	prefix, suffix := n.getSourceNamingRules(info.ImportPath)
	result := strings.TrimPrefix(baseName, prefix)
	result = strings.TrimSuffix(result, suffix)
	
	// Apply target package naming rules
	targetPrefix, targetSuffix := n.getTargetNamingRules(info.ImportPath)
	result = targetPrefix + result + targetSuffix
	
	return result
}

// GetFunctionName returns the function name for converting between two types.
func (n *Namer) GetFunctionName(sourceType, targetType *model.Type) string {
	sourceName := n.getSimpleTypeName(sourceType)
	targetName := n.getSimpleTypeName(targetType)
	return "Convert" + strings.Title(sourceName) + "To" + strings.Title(targetName)
}

// getSourceNamingRules returns the naming rules for the source package.
func (n *Namer) getSourceNamingRules(pkgPath string) (string, string) {
	// Check if this is the source package in any pair
	for sourcePkg, targetPkg := range n.ruleSet.PackagePairs {
		if pkgPath == sourcePkg {
			return n.ruleSet.NamingRules.SourcePrefix, n.ruleSet.NamingRules.SourceSuffix
		}
		if pkgPath == targetPkg {
			return n.ruleSet.NamingRules.TargetPrefix, n.ruleSet.NamingRules.TargetSuffix
		}
	}
	return "", ""
}

// getTargetNamingRules returns the naming rules for the target package.
func (n *Namer) getTargetNamingRules(pkgPath string) (string, string) {
	// This is more complex and depends on conversion direction
	// For now, return empty strings
	return "", ""
}

// getSimpleTypeName returns the simple type name without package prefixes.
func (n *Namer) getSimpleTypeName(t *model.Type) string {
	if t == nil {
		return ""
	}
	
	name := t.Name
	if t.IsPointer {
		name = strings.TrimPrefix(name, "*")
	}
	if t.Kind == model.TypeKindSlice {
		name = strings.TrimPrefix(name, "[]")
	}
	return name
}

// GetAliasName returns the alias name for a given model.Type, applying target naming rules.
func (n *Namer) GetAliasName(t *model.Type) string {
	if t == nil {
		return ""
	}
	baseName := t.Name // Use the base name from the model.Type

	// Apply target package naming rules from the RuleSet
	// This assumes the alias is being generated for a 'target' type.
	result := baseName
	if n.ruleSet.NamingRules.TargetPrefix != "" {
		result = n.ruleSet.NamingRules.TargetPrefix + result
	}
	if n.ruleSet.NamingRules.TargetSuffix != "" {
		result = result + n.ruleSet.NamingRules.TargetSuffix
	}
	return result
}
