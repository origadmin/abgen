package config

import (
	"fmt"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Parser is responsible for parsing abgen directives and building a Config object.
type Parser struct {
	config *Config
}

// NewParser creates a new instance of a Parser.
func NewParser() *Parser {
	return &Parser{
		config: NewConfig(),
	}
}

// Parse is the main entry point for configuration parsing. It takes a source
// directory, loads the initial package, discovers directives, and builds a
// complete Config object.
func (p *Parser) Parse(sourceDir string) (*Config, error) {
	// Set the directive path in the generation context
	p.config.GenerationContext.DirectivePath = sourceDir

	// 1. Load the initial package to find directives
	initialLoaderCfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedSyntax | packages.NeedFiles | packages.NeedModule,
		Dir:   sourceDir,
		Tests: false,
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

	// 2. Discover and parse directives from the loaded package
	return p.parseDirectives(initialPkg)
}

// parseDirectives scans and parses all abgen directives from a given package.
func (p *Parser) parseDirectives(pkg *packages.Package) (*Config, error) {
	if pkg == nil {
		return nil, fmt.Errorf("package context cannot be nil")
	}
	p.config.GenerationContext.PackageName = pkg.Name
	p.config.GenerationContext.PackagePath = pkg.ID

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

	for _, directive := range directives {
		if err := p.parseSingleDirective(directive); err != nil {
			return nil, err
		}
	}

	return p.config, nil
}

// parse processes a single directive string and updates the configuration.
func (p *Parser) parseSingleDirective(directive string) error {
	if !strings.HasPrefix(directive, "//go:abgen:") {
		return nil // Not a valid directive
	}
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
		p.config.GlobalBehaviorRules.GenerateAlias = (value == "true")
	case "convert":
		p.parseConvertRule(value)
	case "convert:rule":
		p.parseCustomFuncRule(value)
	default:
	}

	return nil
}

func (p *Parser) findOrCreateRule(sourceFQN, targetFQN string) *ConversionRule {
	for _, rule := range p.config.ConversionRules {
		if rule.SourceType == sourceFQN && rule.TargetType == targetFQN {
			return rule
		}
	}
	newRule := &ConversionRule{
		SourceType: sourceFQN,
		TargetType: targetFQN,
		Direction:  DirectionBoth, // Default direction
		FieldRules: FieldRuleSet{
			Ignore: make(map[string]struct{}),
			Remap:  make(map[string]string),
		},
	}
	p.config.ConversionRules = append(p.config.ConversionRules, newRule)
	return newRule
}

// parsePackagePath parses "package:path" directives and adds to PackageAliases.
func (p *Parser) parsePackagePath(value string) {
	parts := strings.Split(value, ",")
	path := parts[0]
	alias := ""
	if len(parts) > 1 && strings.HasPrefix(parts[1], "alias=") {
		alias = strings.TrimPrefix(parts[1], "alias=")
	} else {
		alias = path[strings.LastIndex(path, "/")+1:]
	}
	p.config.PackageAliases[alias] = path
}

// resolvePackagePath tries to resolve an identifier as an alias, falling back to treating it as a full path.
func (p *Parser) resolvePackagePath(identifier string) string {
	if path, ok := p.config.PackageAliases[identifier]; ok {
		return path // It's an alias, return the corresponding path.
	}
	return identifier // Not an alias, assume it's a full package path.
}

// parsePackagePairs parses "pair:packages" directives.
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

// parseConvertRule parses "convert" directives using a key-value format.
func (p *Parser) parseConvertRule(value string) {
	parts := strings.Split(value, ",")
	var sourceTypeStr, targetTypeStr string
	var direction, ignore, remap string

	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, val := kv[0], kv[1]

		switch key {
		case "source":
			sourceTypeStr = val
		case "target":
			targetTypeStr = val
		case "direction":
			direction = val
		case "ignore":
			ignore = val
		case "remap":
			remap = val
		}
	}

	if sourceTypeStr == "" || targetTypeStr == "" {
		return
	}

	sourceFQN := p.resolveTypeFQN(sourceTypeStr)
	targetFQN := p.resolveTypeFQN(targetTypeStr)
	rule := p.findOrCreateRule(sourceFQN, targetFQN)

	if direction == "oneway" {
		rule.Direction = DirectionOneway
	}
	if ignore != "" {
		for _, field := range strings.Split(ignore, ";") {
			rule.FieldRules.Ignore[field] = struct{}{}
		}
	}
	if remap != "" {
		for _, remapPair := range strings.Split(remap, ";") {
			fromTo := strings.SplitN(remapPair, ":", 2)
			if len(fromTo) == 2 {
				rule.FieldRules.Remap[fromTo[0]] = fromTo[1]
			}
		}
	}
}

// parseCustomFuncRule parses "convert:rule" directives.
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

	if source == "" || target == "" || funcName == "" {
		return
	}

	sourceFQN := p.resolveTypeFQN(source)
	targetFQN := p.resolveTypeFQN(target)
	rule := p.findOrCreateRule(sourceFQN, targetFQN)
	rule.CustomFunc = funcName
}

// resolveTypeFQN resolves a type string (e.g., "alias.TypeName" or "path/to/pkg.TypeName") to a fully-qualified name.
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
