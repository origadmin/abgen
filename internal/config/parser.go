package config

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	slog "log/slog"
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
		Mode:       packages.NeedName | packages.NeedSyntax | packages.NeedFiles | packages.NeedModule | packages.NeedTypes | packages.NeedTypesInfo,
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

	p.extractExistingAliases(pkg)

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

	// No directives is not an error, it might just mean no conversions are needed.
	// The generator should handle this gracefully.
	if len(directives) == 0 {
		slog.Warn("no abgen directives found, no code will be generated", "package", pkg.ID)
		return p.config, nil
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
	slog.Debug("Registered package alias", "alias", alias, "path", path)
}

func (p *Parser) resolvePackagePath(identifier string) string {
	if path, ok := p.config.PackageAliases[identifier]; ok {
		return path
	}
	// Assume it's a full path if not found in aliases
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
		// Assume it's a type in the current package if no package identifier is present
		return p.config.GenerationContext.PackagePath + "." + typeStr
	}
	packageIdentifier := typeStr[:lastDot]
	typeName := typeStr[lastDot+1:]
	packagePath := p.resolvePackagePath(packageIdentifier)
	return packagePath + "." + typeName
}

// extractExistingAliases extracts type aliases from the source code
func (p *Parser) extractExistingAliases(pkg *packages.Package) {
	if pkg.TypesInfo == nil {
		return
	}

	// Also check for type aliases in type declarations (GenDecl)
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
				for _, spec := range genDecl.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						// Check if this is a type alias (typeSpec.Assign != 0)
						if typeSpec.Assign != 0 {
							// This is a type alias, try to resolve the RHS
							if selExpr, ok := typeSpec.Type.(*ast.SelectorExpr); ok {
								// Handle qualified identifiers like "ent.User"
								if pkgName, ok := selExpr.X.(*ast.Ident); ok {
									// First, try to resolve using PackageAliases
									if pkgPath, exists := p.config.PackageAliases[pkgName.Name]; exists {
										typeName := selExpr.Sel.Name
										fqn := pkgPath + "." + typeName
										p.config.ExistingAliases[typeSpec.Name.Name] = fqn
									} else if obj := pkg.TypesInfo.Uses[pkgName]; obj != nil && obj.Pkg() != nil {
										if obj := pkg.TypesInfo.Uses[pkgName]; obj != nil && obj.Pkg() != nil {
											pkgPath := obj.Pkg().Path()
											typeName := selExpr.Sel.Name
											fqn := pkgPath + "." + typeName
											p.config.ExistingAliases[typeSpec.Name.Name] = fqn
										} else if obj := pkg.TypesInfo.Defs[pkgName]; obj != nil && obj.Pkg() != nil {
											// Try in Definitions as well
											pkgPath := obj.Pkg().Path()
											typeName := selExpr.Sel.Name
											fqn := pkgPath + "." + typeName
											p.config.ExistingAliases[typeSpec.Name.Name] = fqn
										} else {
											// Fallback: look for the package in imports
											for _, imp := range pkg.Imports {
												if imp.Name == pkgName.Name {
													typeName := selExpr.Sel.Name
													fqn := imp.PkgPath + "." + typeName
													p.config.ExistingAliases[typeSpec.Name.Name] = fqn
													break
												}
											}
										}
									}
								} else if ident, ok := typeSpec.Type.(*ast.Ident); ok {
									// Handle simple identifiers
									if obj := pkg.TypesInfo.Uses[ident]; obj != nil {
										if named, ok := obj.Type().(*types.Named); ok && named.Obj() != nil && named.Obj().Pkg() != nil {
											fqn := named.Obj().Pkg().Path() + "." + named.Obj().Name()
											p.config.ExistingAliases[typeSpec.Name.Name] = fqn
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
}
