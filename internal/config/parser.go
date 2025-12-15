package config

import (
	"fmt"

	"strings"

	"golang.org/x/tools/go/packages"
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

// Parse is the main entry point for configuration parsing.
func (p *Parser) Parse(sourceDir string) (*Config, error) {
	p.config.GenerationContext.DirectivePath = sourceDir

	initialLoaderCfg := &packages.Config{
		Mode:       packages.NeedName | packages.NeedSyntax | packages.NeedFiles | packages.NeedModule | packages.NeedTypes | packages.NeedTypesInfo, // FIX: Added packages.NeedTypesInfo
		Dir:        sourceDir,
		Tests:      false,
		BuildFlags: []string{"-tags=abgen_source"},
	}

	initialPkgs, err := packages.Load(initialLoaderCfg, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to load initial package at %s: %w", sourceDir, err)
	}
	if packages.PrintErrors(initialPkgs) > 0 {
		return nil, fmt.Errorf("initial package at %s contains errors", sourceDir)
	}
	if len(initialPkgs) == 0 {
		return nil, fmt.Errorf("no initial package found at %s", sourceDir)
	}
	initialPkg := initialPkgs[0]

	return p.parseDirectives(initialPkg)
}

// parseDirectives scans and parses all abgen directives and existing type aliases.
func (p *Parser) parseDirectives(pkg *packages.Package) (*Config, error) {
	if pkg == nil {
		return nil, fmt.Errorf("package context cannot be nil")
	}
	p.config.GenerationContext.PackageName = pkg.Name
	p.config.GenerationContext.PackagePath = pkg.ID

	var directives []string
	for _, file := range pkg.Syntax {
		// Collect directives
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

	for _, directive := range directives {
		if err := p.parseSingleDirective(directive); err != nil {
			return nil, err
		}
	}

	p.mergeCustomFuncRules()

	return p.config, nil
}

// parseSingleDirective processes a single directive string.
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
		return
	}
	sourceIdentifier, targetIdentifier := strings.TrimSpace(pair[0]), strings.TrimSpace(pair[1])
	sourcePath := p.resolvePackagePath(sourceIdentifier)
	targetPath := p.resolvePackagePath(targetIdentifier)
	if sourcePath != "" && targetPath != "" {
		p.config.PackagePairs = append(p.config.PackagePairs, &PackagePair{SourcePath: sourcePath, TargetPath: targetPath})
	}
}

func (p *Parser) parseConvertRule(value string) {
	parts := strings.Split(value, ",")
	rule := &ConversionRule{
		Direction: DirectionBoth,
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
		mapKey := sourceFQN + "->" + targetFQN
		p.config.CustomFunctionRules[mapKey] = funcName
	}
}

func (p *Parser) mergeCustomFuncRules() {
	for _, rule := range p.config.ConversionRules {
		key := rule.SourceType + "->" + rule.TargetType
		if funcName, ok := p.config.CustomFunctionRules[key]; ok {
			rule.CustomFunc = funcName
		}
	}
}

func (p *Parser) resolveTypeFQN(typeStr string) string {
	lastDot := strings.LastIndex(typeStr, ".")
	if lastDot == -1 {
		return typeStr
	}
	packageIdentifier := typeStr[:lastDot]
	typeName := typeStr[lastDot+1:]
	packagePath := p.resolvePackagePath(packageIdentifier)
	return packagePath + "." + typeName
}
