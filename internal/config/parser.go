package config

import (
	"fmt"
	"log/slog"
	"strings"
)

// defaultPackageAliases provides a set of commonly used packages.
var defaultPackageAliases = map[string]string{
	"time": "time",
	"uuid": "github.com/google/uuid",
	"json": "encoding/json",
}

// Parser is responsible for parsing abgen directives and building a Config object.
type Parser struct {
	config *Config
}

// NewParser creates a new instance of a Parser.
func NewParser() *Parser {
	config := NewConfig()
	for alias, path := range defaultPackageAliases {
		config.PackageAliases[alias] = path
	}
	return &Parser{
		config: config,
	}
}

// ParseDirectives processes a list of directive strings and populates the configuration.
// It uses a two-pass approach to ensure package aliases are resolved before other rules.
func (p *Parser) ParseDirectives(directives []string, currentPkgName, currentPkgPath string) (*Config, error) {
	p.config.GenerationContext.PackageName = currentPkgName
	p.config.GenerationContext.PackagePath = currentPkgPath

	if len(directives) == 0 {
		slog.Warn("no abgen directives found, no code will be generated", "package", currentPkgPath)
		return p.config, nil
	}

	// First pass: Process only package:path directives to populate aliases.
	for _, directive := range directives {
		if strings.Contains(directive, "package:path") {
			if err := p.parseSingleDirective(directive); err != nil {
				return nil, err
			}
		}
	}

	// Second pass: Process all other directives.
	for _, directive := range directives {
		if !strings.Contains(directive, "package:path") {
			if err := p.parseSingleDirective(directive); err != nil {
				return nil, err
			}
		}
	}

	p.mergeCustomFuncRules()

	return p.config, nil
}

// parseSingleDirective parses a single directive string and updates the config.
func (p *Parser) parseSingleDirective(directive string) error {
	directive = strings.TrimPrefix(directive, "//go:abgen:")
	parts := strings.SplitN(directive, "=", 2)
	key := parts[0]
	var value string
	if len(parts) > 1 {
		value = strings.Trim(parts[1], `"`)
	}

	switch key {
	case "package:path":
		p.parsePackagePath(value)
	case "pair:packages":
		p.parsePackagePairs(value)
	case "convert:source:suffix":
		p.config.NamingRules.SourceSuffix = value
	case "convert:target:suffix":
		p.config.NamingRules.TargetSuffix = value
	case "convert:source:prefix":
		p.config.NamingRules.SourcePrefix = value
	case "convert:target:prefix":
		p.config.NamingRules.TargetPrefix = value
	case "convert:alias:generate":
		p.config.GlobalBehaviorRules.GenerateAlias = value == "true"
	case "convert:direction":
		if value == "oneway" {
			p.config.GlobalBehaviorRules.DefaultDirection = DirectionOneway
		} else {
			p.config.GlobalBehaviorRules.DefaultDirection = DirectionBoth
		}
	case "convert":
		p.parseConvertRule(value)
	case "convert:rule":
		p.parseCustomFuncRule(value)
	}
	return nil
}

func (p *Parser) parsePackagePath(value string) {
	parts := strings.Split(value, ",")
	path := parts[0]
	var alias string
	if len(parts) > 1 && strings.HasPrefix(parts[1], "alias=") {
		alias = strings.TrimPrefix(parts[1], "alias=")
	} else {
		alias = path[strings.LastIndex(path, "/")+1:]
	}
	p.config.PackageAliases[alias] = path
	slog.Debug("Parser.parsePackagePath after processing", "alias", alias, "path", path, "allPackageAliases", p.config.PackageAliases)
}

func (p *Parser) resolvePackagePath(identifier string) string {
	if path, ok := p.config.PackageAliases[identifier]; ok {
		return path
	}
	return identifier
}

func (p *Parser) parsePackagePairs(value string) {
	pair := strings.Split(value, ",")
	if len(pair) != 2 {
		slog.Warn("invalid pair:packages directive, expected two comma-separated values", "value", value)
		return
	}
	sourceIdentifier, targetIdentifier := strings.TrimSpace(pair[0]), strings.TrimSpace(pair[1])
	sourcePath := p.resolvePackagePath(sourceIdentifier)
	targetPath := p.resolvePackagePath(targetIdentifier)
	if sourcePath != "" && targetPath != "" {
		p.config.PackagePairs = append(p.config.PackagePairs, &PackagePair{SourcePath: sourcePath, TargetPath: targetPath})
		slog.Debug("Registered package pair", "source", sourcePath, "target", targetPath)
	}
}

func (p *Parser) parseConvertRule(value string) {
	parts := strings.Split(value, ",")
	rule := &ConversionRule{
		Direction: p.config.GlobalBehaviorRules.DefaultDirection,
		FieldRules: FieldRuleSet{
			Ignore: make(map[string]struct{}),
			Remap:  make(map[string]string),
		},
	}
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, val := kv[0], kv[1]
		switch key {
		case "source":
			rule.SourceType = p.resolveTypeFQN(val)
		case "target":
			rule.TargetType = p.resolveTypeFQN(val)
		case "direction":
			if val == "oneway" {
				rule.Direction = DirectionOneway
			} else if val == "both" {
				rule.Direction = DirectionBoth
			}
		case "ignore":
			for _, field := range strings.Split(val, ";") {
				rule.FieldRules.Ignore[field] = struct{}{}
			}
		case "remap":
			for _, remapPair := range strings.Split(val, ";") {
				fromTo := strings.SplitN(remapPair, ":", 2)
				if len(fromTo) == 2 {
					rule.FieldRules.Remap[fromTo[0]] = fromTo[1]
				}
			}
		}
	}
	if rule.SourceType != "" && rule.TargetType != "" {
		p.config.ConversionRules = append(p.config.ConversionRules, rule)
	}
}

func (p *Parser) parseCustomFuncRule(value string) {
	parts := strings.Split(value, ",")
	var source, target, funcName string
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), ":", 2)
		if len(kv) != 2 {
			continue
		}
		key, val := kv[0], kv[1]
		switch key {
		case "source":
			source = val
		case "target":
			target = val
		case "func":
			funcName = val
		}
	}
	if source != "" && target != "" && funcName != "" {
		sourceFQN := p.resolveTypeFQN(source)
		targetFQN := p.resolveTypeFQN(target)
		mapKey := fmt.Sprintf("%s->%s", sourceFQN, targetFQN)
		p.config.CustomFunctionRules[mapKey] = funcName
	}
}

func (p *Parser) mergeCustomFuncRules() {
	for _, rule := range p.config.ConversionRules {
		key := fmt.Sprintf("%s->%s", rule.SourceType, rule.TargetType)
		if funcName, ok := p.config.CustomFunctionRules[key]; ok {
			rule.CustomFunc = funcName
		}
	}
}

func (p *Parser) resolveTypeFQN(typeStr string) string {
	lastDot := strings.LastIndex(typeStr, ".")
	if lastDot == -1 {
		if p.config.GenerationContext.PackagePath != "" {
			return p.config.GenerationContext.PackagePath + "." + typeStr
		}
		return typeStr
	}
	packageIdentifier := typeStr[:lastDot]
	typeName := typeStr[lastDot+1:]
	packagePath := p.resolvePackagePath(packageIdentifier)
	return packagePath + "." + typeName
}
