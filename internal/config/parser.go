// Package config provides directive parsing capabilities for abgen.
package config

import (
	"fmt"
	"strings"

	"golang.org/x/tools/go/packages"
)

// DirectiveParser is responsible for discovering abgen directives from Go source files.
type DirectiveParser struct{}

// NewDirectiveParser creates a new instance of DirectiveParser.
func NewDirectiveParser() *DirectiveParser {
	return &DirectiveParser{}
}

// DiscoverDirectives scans the provided package for //go:abgen: comments and extracts them.
func (dp *DirectiveParser) DiscoverDirectives(pkg *packages.Package) ([]string, error) {
	var directives []string
	for _, file := range pkg.Syntax {
		for _, commentGroup := range file.Comments {
			for _, comment := range commentGroup.List {
				if strings.HasPrefix(comment.Text, "//go:abgen:") {
					directives = append(directives, strings.TrimSpace(comment.Text))
				}
			}
		}
	}
	if len(directives) == 0 {
		return nil, fmt.Errorf("no abgen directives found in package %s", pkg.ID)
	}
	return directives, nil
}

// ExtractDependencies extracts package import paths from directives.
// This method is a simplified placeholder and might need more sophisticated parsing
// depending on the directive format for dependencies.
func (dp *DirectiveParser) ExtractDependencies(directives []string) []string {
	// This is a simplified implementation. A more robust solution would parse
	// specific directive types that declare dependencies.
	// For now, we'll look for 'package:path' directives.
	var dependencies []string
	for _, d := range directives {
		if strings.HasPrefix(d, "//go:abgen:package:path=") {
			// Example: //go:abgen:package:path=github.com/origadmin/abgen/testdata/fixture/ent,alias=ent
			parts := strings.SplitN(d, "=", 2)
			if len(parts) == 2 {
				pathAndAlias := strings.SplitN(parts[1], ",", 2)
				dependencies = append(dependencies, pathAndAlias[0])
			}
		}
		// Add other directive types that might declare dependencies if needed
	}
	return dependencies
}

// RuleParser holds the state for the parsing process of abgen rules.
type RuleParser struct {
	ruleSet        *RuleSet
	packageAliases map[string]string // Maps alias to full package path
}

// NewRuleParser creates a new RuleParser instance.
func NewRuleParser() *RuleParser {
	return &RuleParser{
		ruleSet:        NewRuleSet(),
		packageAliases: make(map[string]string),
	}
}

// Parse parses a single directive string and updates the ruleset.
func (rp *RuleParser) Parse(directive string) error {
	// Remove the //go:abgen: prefix if present
	if strings.HasPrefix(directive, "//go:abgen:") {
		directive = strings.TrimPrefix(directive, "//go:abgen:")
	}

	parts := strings.SplitN(directive, "=", 2) // Use SplitN for robustness
	key := parts[0]
	var value string
	if len(parts) > 1 {
		value = strings.Trim(parts[1], `"`)
	}

	// Parse based on the exact key
	switch key {
	case "package:path":
		rp.parsePackagePath(value)
	case "pair:packages":
		rp.parsePackagePairs(value)
	case "convert:source:suffix":
		rp.ruleSet.NamingRules.SourceSuffix = value
	case "convert:target:suffix":
		rp.ruleSet.NamingRules.TargetSuffix = value
	case "convert:source:prefix":
		rp.ruleSet.NamingRules.SourcePrefix = value
	case "convert:target:prefix":
		rp.ruleSet.NamingRules.TargetPrefix = value
	case "convert:direction":
		// This requires type context, simplified for now
		rp.ruleSet.BehaviorRules.Direction["*"] = value
	case "convert:alias:generate":
		rp.ruleSet.BehaviorRules.GenerateAlias = (value == "true")
	case "convert:ignore":
		rp.parseIgnoreRule(value)
	case "convert:remap":
		rp.parseRemapRule(value)
	case "convert":
		// This directive is currently unhandled. It seems to be a remnant of a
		// previous design. The current logic relies on package pairs and automatic
		// type discovery. We will ignore it for now.
	}

	return nil
}

// ParseDirectives parses multiple directives and sets the generation context.
func (rp *RuleParser) ParseDirectives(directives []string, pkg *packages.Package) error {
	if pkg == nil {
		return fmt.Errorf("package context cannot be nil")
	}
	rp.ruleSet.Context.PackageName = pkg.Name
	rp.ruleSet.Context.PackagePath = pkg.ID

	for _, directive := range directives {
		if err := rp.Parse(directive); err != nil {
			return err
		}
	}
	return nil
}

// GetRuleSet returns the final, consolidated RuleSet.
func (rp *RuleParser) GetRuleSet() *RuleSet {
	return rp.ruleSet
}

// parsePackagePath parses package path directives.
func (rp *RuleParser) parsePackagePath(value string) {
	pathAliasParts := strings.Split(value, ",")
	path := pathAliasParts[0]
	alias := ""
	if len(pathAliasParts) > 1 && strings.HasPrefix(pathAliasParts[1], "alias=") {
		alias = strings.TrimPrefix(pathAliasParts[1], "alias=")
	} else {
		alias = path[strings.LastIndex(path, "/")+1:]
	}
	rp.packageAliases[alias] = path
}

// parsePackagePairs parses package pairing directives.
func (rp *RuleParser) parsePackagePairs(value string) {
	pair := strings.Split(value, ",")
	if len(pair) == 2 {
		sourceAlias, targetAlias := pair[0], pair[1]
		if sourcePath, ok := rp.packageAliases[sourceAlias]; ok {
			if targetPath, ok := rp.packageAliases[targetAlias]; ok {
				rp.ruleSet.PackagePairs[sourcePath] = targetPath
			}
		}
	}
}

// parseIgnoreRule parses field ignore rules.
func (rp *RuleParser) parseIgnoreRule(value string) {
	parts := strings.Split(value, "#")
	if len(parts) != 2 {
		return
	}
	typeName, fieldStr := parts[0], parts[1]
	fields := strings.Split(fieldStr, ",")

	if _, ok := rp.ruleSet.FieldRules.Ignore[typeName]; !ok {
		rp.ruleSet.FieldRules.Ignore[typeName] = make(map[string]struct{})
	}
	for _, field := range fields {
		rp.ruleSet.FieldRules.Ignore[typeName][field] = struct{}{}
	}
}

// parseRemapRule parses field remap rules.
func (rp *RuleParser) parseRemapRule(value string) {
	parts := strings.Split(value, "#")
	if len(parts) != 2 {
		return
	}
	typeName, remapStr := parts[0], parts[1]

	remapParts := strings.Split(remapStr, ":")
	if len(remapParts) != 2 {
		return
	}
	sourceField, targetField := remapParts[0], remapParts[1]

	if _, ok := rp.ruleSet.FieldRules.Remap[typeName]; !ok {
		rp.ruleSet.FieldRules.Remap[typeName] = make(map[string]string)
	}
	rp.ruleSet.FieldRules.Remap[typeName][sourceField] = targetField
}
