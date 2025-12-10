// Package config provides directive parsing capabilities for abgen.
package config

import (
	"strings"
)

// Parser holds the state for the parsing process.
type Parser struct {
	ruleSet        *RuleSet
	packageAliases map[string]string // Maps alias to full package path
}

// NewParser creates a new parser instance.
func NewParser() *Parser {
	return &Parser{
		ruleSet:        NewRuleSet(),
		packageAliases: make(map[string]string),
	}
}

// Parse parses a single directive string and updates the ruleset.
func (p *Parser) Parse(directive string) error {
	// Remove the //go:abgen: prefix if present
	if strings.HasPrefix(directive, "//go:abgen:") {
		directive = strings.TrimPrefix(directive, "//go:abgen:")
	}
	
	parts := strings.Split(directive, "=")
	key := parts[0]
	var value string
	if len(parts) > 1 {
		value = strings.Trim(parts[1], `"`)
	}

	// Parse based on key
	switch {
	case strings.HasPrefix(key, "package:path"):
		p.parsePackagePath(value)
	case strings.HasPrefix(key, "pair:packages"):
		p.parsePackagePairs(value)
	case strings.HasPrefix(key, "convert:source:suffix"):
		p.ruleSet.NamingRules.SourceSuffix = value
	case strings.HasPrefix(key, "convert:target:suffix"):
		p.ruleSet.NamingRules.TargetSuffix = value
	case strings.HasPrefix(key, "convert:source:prefix"):
		p.ruleSet.NamingRules.SourcePrefix = value
	case strings.HasPrefix(key, "convert:target:prefix"):
		p.ruleSet.NamingRules.TargetPrefix = value
	case strings.HasPrefix(key, "convert:direction"):
		// This requires type context, simplified for now
		p.ruleSet.BehaviorRules.Direction["*"] = value
	case strings.HasPrefix(key, "convert:alias:generate"):
		p.ruleSet.BehaviorRules.GenerateAlias = (value == "true")
	case strings.HasPrefix(key, "convert:ignore"):
		p.parseIgnoreRule(value)
	case strings.HasPrefix(key, "convert:remap"):
		p.parseRemapRule(value)
	}
	
	return nil
}

// ParseDirectives parses multiple directives.
func (p *Parser) ParseDirectives(directives []string) error {
	for _, directive := range directives {
		if err := p.Parse(directive); err != nil {
			return err
		}
	}
	return nil
}

// GetRuleSet returns the final, consolidated RuleSet.
func (p *Parser) GetRuleSet() *RuleSet {
	return p.ruleSet
}

// parsePackagePath parses package path directives.
func (p *Parser) parsePackagePath(value string) {
	pathAliasParts := strings.Split(value, ",")
	path := pathAliasParts[0]
	alias := ""
	if len(pathAliasParts) > 1 && strings.HasPrefix(pathAliasParts[1], "alias=") {
		alias = strings.TrimPrefix(pathAliasParts[1], "alias=")
	} else {
		alias = path[strings.LastIndex(path, "/")+1:]
	}
	p.packageAliases[alias] = path
}

// parsePackagePairs parses package pairing directives.
func (p *Parser) parsePackagePairs(value string) {
	pair := strings.Split(value, ",")
	if len(pair) == 2 {
		sourceAlias, targetAlias := pair[0], pair[1]
		if sourcePath, ok := p.packageAliases[sourceAlias]; ok {
			if targetPath, ok := p.packageAliases[targetAlias]; ok {
				p.ruleSet.PackagePairs[sourcePath] = targetPath
			}
		}
	}
}

// parseIgnoreRule parses field ignore rules.
func (p *Parser) parseIgnoreRule(value string) {
	parts := strings.Split(value, "#")
	if len(parts) != 2 {
		return
	}
	typeName, fieldStr := parts[0], parts[1]
	fields := strings.Split(fieldStr, ",")

	if _, ok := p.ruleSet.FieldRules.Ignore[typeName]; !ok {
		p.ruleSet.FieldRules.Ignore[typeName] = make(map[string]struct{})
	}
	for _, field := range fields {
		p.ruleSet.FieldRules.Ignore[typeName][field] = struct{}{}
	}
}

// parseRemapRule parses field remap rules.
func (p *Parser) parseRemapRule(value string) {
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

	if _, ok := p.ruleSet.FieldRules.Remap[typeName]; !ok {
		p.ruleSet.FieldRules.Remap[typeName] = make(map[string]string)
	}
	p.ruleSet.FieldRules.Remap[typeName][sourceField] = targetField
}