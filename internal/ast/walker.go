// Package ast implements the functions, types, and interfaces for the module.
package ast

import (
	"fmt"
	goast "go/ast"
	"go/token"
	gotypes "go/types"
	"log/slog"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/types"
)

// PackageWalker 包遍历器

type PackageWalker struct {
	imports     map[string]string
	pathAliases map[string]string // Maps alias to full package path

	rules map[string]*types.ConversionConfig // The global rule set, key: SourceFQN->TargetFQN

	currentPkg *packages.Package

	typeCache map[string]*types.TypeInfo // Changed to pointer

	loadedPkgs map[string]*packages.Package // 缓存已加载的包

	packageMode packages.LoadMode

	defaultConfig *types.ConversionConfig // Global default config, affected by file-level directives

	allKnownPkgs []*packages.Package // New field to hold all packages known to the resolver

	localTypeNameToFQN map[string]string // Maps local type names to their FQN

}

// AddPackages adds more *packages.Package instances to the walker's known packages.
func (w *PackageWalker) AddPackages(pkgs ...*packages.Package) {
	existingPkgs := make(map[string]bool)
	for _, p := range w.allKnownPkgs {
		existingPkgs[p.PkgPath] = true
	}

	for _, newPkg := range pkgs {
		if !existingPkgs[newPkg.PkgPath] {
			w.allKnownPkgs = append(w.allKnownPkgs, newPkg)
			existingPkgs[newPkg.PkgPath] = true
		}
	}
}

// exprToString resolves an expression to its fully qualified type name.
func (w *PackageWalker) exprToString(expr goast.Expr, pkg *packages.Package) string {
	if pkg == nil || pkg.TypesInfo == nil {
		return fmt.Sprintf("<missing_type_info_for_%T>", expr)
	}

	tv, ok := pkg.TypesInfo.Types[expr]
	if !ok || tv.Type == nil {
		if ident, isIdent := expr.(*goast.Ident); isIdent {
			return ident.Name
		}
		return fmt.Sprintf("<unresolved_type_%T>", expr)
	}

	qualifier := func(p *gotypes.Package) string {
		// First, check if there is a package path alias
		for alias, path := range w.pathAliases {
			if path == p.Path() {
				return alias // Use the directive alias
			}
		}
		// Then, check the regular imports
		for _, knownPkg := range w.allKnownPkgs {
			if knownPkg.Types.Path() == p.Path() {
				return knownPkg.PkgPath
			}
		}
		return p.Path()
	}

	return gotypes.TypeString(tv.Type, qualifier)
}

func (w *PackageWalker) GetTypeCache() map[string]*types.TypeInfo { // Changed return type

	return w.typeCache

}

func (w *PackageWalker) GetLocalTypeNameToFQN() map[string]string {

	return w.localTypeNameToFQN

}

// GetCurrentPackage returns the package currently being processed by the walker.
func (w *PackageWalker) GetCurrentPackage() *packages.Package {
	return w.currentPkg
}

// NewPackageWalker 创建新的遍历器

func NewPackageWalker() *PackageWalker { // Removed graph parameter
	return &PackageWalker{
		rules: make(map[string]*types.ConversionConfig), // NEW: Use rules map instead of graph

		imports:     make(map[string]string),
		pathAliases: make(map[string]string),

		typeCache: make(map[string]*types.TypeInfo), // Changed initialization

		loadedPkgs: make(map[string]*packages.Package),

		packageMode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,

		defaultConfig: &types.ConversionConfig{ // NEW: Initialize default config
			Direction:           "both",
			IgnoreFields:        make(map[string]bool),
			IgnoreTypes:         make(map[string]bool),
			RemapFields:         make(map[string]string),
			TypeConversionRules: make([]types.TypeConversionRule, 0),
		},

		localTypeNameToFQN: make(map[string]string),
	}
}

// Resolve is a public method, for resolving type information

func (w *PackageWalker) Resolve(typeName string) (*types.TypeInfo, error) { // Changed return type to *types.TypeInfo

	typeName = strings.TrimPrefix(typeName, "*")

	info := w.resolveTargetType(typeName)

	if info == nil || info.Name == "" { // Check for nil info

		return nil, fmt.Errorf("type %q not found", typeName) // Return nil pointer

	}

	return info, nil // Return pointer

}

// DiscoverPackages performs the first pass of analysis on a directive package.
// Its sole purpose is to find `pair:packages` directives and return a list
// of unique package paths that need to be loaded for the full analysis.
func (w *PackageWalker) DiscoverPackages(pkg *packages.Package) ([]string, error) {
	slog.Info("开始发现阶段", "包", pkg.PkgPath)
	w.currentPkg = pkg

	discoveredPaths := make(map[string]struct{})

	for _, file := range pkg.Syntax {
		filename := pkg.Fset.File(file.Pos()).Name()
		if strings.HasSuffix(filepath.Base(filename), ".gen.go") {
			slog.Debug("DiscoverPackages: Skipping generated file", "file", filename)
			continue
		}

		for _, comment := range file.Comments {
			for _, line := range comment.List {
				key, value := parseDirective(line.Text)
				if key != "pair:packages" {
					continue
				}

				if value != "" {
					paths := strings.Split(value, ",")
					for _, p := range paths {
						path := strings.TrimSpace(p)
						if resolvedPath, ok := w.pathAliases[path]; ok {
							path = resolvedPath
						}
						discoveredPaths[path] = struct{}{}
					}
				}
			}
		}
	}

	pathList := make([]string, 0, len(discoveredPaths))
	for p := range discoveredPaths {
		pathList = append(pathList, p)
	}
	slog.Info("发现阶段完成", "发现的包", pathList)
	return pathList, nil
}

func (w *PackageWalker) Analyze(pkg *packages.Package) error {
	slog.Info("开始分析阶段", "包", pkg.PkgPath)

	w.currentPkg = pkg
	w.collectImports(pkg.Syntax)
	slog.Debug("Analyze: collected imports", "imports", w.imports)

	// The caller is now responsible for adding all necessary packages beforehand.
	// We ensure the current package is in the list.
	w.AddPackages(pkg)

	slog.Debug("Analyze: allKnownPkgs", "allKnownPkgs", func() []string {
		paths := make([]string, len(w.allKnownPkgs))
		for i, p := range w.allKnownPkgs {
			paths[i] = p.PkgPath
		}
		return paths
	}())

	slog.Debug("Analyze: Number of files in pkg.Syntax", "count", len(pkg.Syntax))
	for _, file := range pkg.Syntax {
		filename := pkg.Fset.File(file.Pos()).Name()
		slog.Info("分析文件", "文件", filename)

		if strings.HasSuffix(filepath.Base(filename), ".gen.go") {
			slog.Debug("Analyze: Skipping generated file", "file", filename)
			continue
		}

		if err := w.processFileDecls(file); err != nil {
			return err
		}
	}
	// After processing all directives, we now process the package pairings.
	if err := w.ProcessPackagePairs(); err != nil {
		return fmt.Errorf("error processing package pairs: %w", err)
	}

	slog.Debug("Analyze: Finished processing package", "pkgPath", pkg.PkgPath)
	return nil
}

func (w *PackageWalker) processFileDecls(file *goast.File) error {

	slog.Debug("processFileDecls: Processing file", "filename", w.currentPkg.Fset.File(file.Pos()).Name())

	w.collectLocalTypeAliases(file)
	w.processCommentDirectives(file)

	return nil
}

// collectLocalTypeAliases scans the file for all type declarations and aliases,
// populating the walker's localTypeNameToFQN map. This corresponds to Pass 1.
func (w *PackageWalker) collectLocalTypeAliases(file *goast.File) {
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*goast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*goast.TypeSpec); ok {
					var fqn string
					// Resolve the TypeInfo for the actual underlying type
					underlyingTypeInfo := w.buildTypeInfoFromTypeSpec(typeSpec.Type, w.currentPkg) // Changed here
					if underlyingTypeInfo.Name != "" {
						if underlyingTypeInfo.ImportPath == "builtin" {
							fqn = underlyingTypeInfo.Name
						} else {
							fqn = underlyingTypeInfo.ImportPath + "." + underlyingTypeInfo.Name
						}
					} else {
						fqn = w.exprToString(typeSpec.Type, w.currentPkg) // Fallback
					}

					w.localTypeNameToFQN[typeSpec.Name.Name] = fqn
					// Create a TypeInfo entry for the local alias itself
					info := w.buildTypeInfoFromTypeSpec(typeSpec, w.currentPkg) // Get info for the alias itself
					info.LocalAlias = typeSpec.Name.Name
					info.IsAlias = true
					info.AliasFor = fqn                     // The FQN of the underlying type
					w.typeCache[typeSpec.Name.Name] = &info // Store pointer
					slog.Debug("collectLocalTypeAliases: Collected local type", "localName", typeSpec.Name.Name, "fqn", fqn)
				}
			}
		}
	}
}

// processCommentDirectives scans all comment groups in a file and processes any
// abgen directives found. This corresponds to Pass 2.
func (w *PackageWalker) processCommentDirectives(file *goast.File) {
	slog.Debug("processCommentDirectives: Number of comment groups", "count", len(file.Comments))
	for _, commentGroup := range file.Comments {
		var definingDirective string
		var modifierDirectives []string
		isDirectiveGroup := false

		for _, comment := range commentGroup.List {
			line := comment.Text
			key, _ := parseDirective(line)
			if key == "" {
				continue
			}

			isDirectiveGroup = true
			if key == "pair:packages" || key == "convert" {
				definingDirective = line
			} else {
				modifierDirectives = append(modifierDirectives, line)
			}
		}

		if !isDirectiveGroup {
			continue
		}

		// It's a file-level directive group, not attached to a type.
		// These directives modify the defaultConfig or define package-level aliases.
		// `convert="A,B"` at file-level creates a specific rule.
		definingKey, _ := parseDirective(definingDirective)
		isConvertDirective := definingKey == "convert"

		if isConvertDirective {
			// Create a new specific conversion rule from this file-level directive
			typeCfg := w.defaultConfig.Clone()
			w.parseAndApplyDirective(definingDirective, typeCfg)
			for _, mod := range modifierDirectives {
				w.parseAndApplyDirective(mod, typeCfg)
			}
			if typeCfg.Source != nil && typeCfg.Target != nil && typeCfg.Source.Type != "" && typeCfg.Target.Type != "" {
				w.AddConversion(typeCfg)
				slog.Debug("processCommentDirectives: Added file-level conversion config", "source", typeCfg.Source.Type, "target", typeCfg.Target.Type)
			}
		} else {
			// All other file-level directives apply to the defaultConfig
			w.applyFileLevelDirectives(definingDirective, modifierDirectives)
		}
	}
}

// applyFileLevelDirectives applies directives that are not attached to a specific type.
func (w *PackageWalker) applyFileLevelDirectives(definingDirective string, modifierDirectives []string) {
	slog.Debug("applyFileLevelDirectives: Applying file-level directives")
	// All file-level directives (pair:packages, global modifiers) update the defaultConfig
	if definingDirective != "" {
		w.parseAndApplyDirective(definingDirective, w.defaultConfig) // Pass defaultConfig
	}
	for _, mod := range modifierDirectives {
		w.parseAndApplyDirective(mod, w.defaultConfig) // Pass defaultConfig
	}
}

// applyTypeLevelDirectives applies directives found in a comment group associated with a type declaration.
func (w *PackageWalker) applyTypeLevelDirectives(file *goast.File, commentGroup *goast.CommentGroup, definingDirective string, modifierDirectives []string, associatedDecl *goast.GenDecl) {
	slog.Debug("applyTypeLevelDirectives: Applying type-level directives")

	for _, spec := range associatedDecl.Specs {
		if typeSpec, ok := spec.(*goast.TypeSpec); ok {
			// Resolve the source type from the type declaration this directive is attached to.
			sourceTypeInfo := w.resolveTargetType(w.exprToString(typeSpec.Type, w.currentPkg))
			effectiveSourceType := ""
			if sourceTypeInfo != nil {
				effectiveSourceType = sourceTypeInfo.ImportPath + "." + sourceTypeInfo.Name
			} else {
				effectiveSourceType = w.exprToString(typeSpec.Type, w.currentPkg) // Fallback
			}

			localAliasForSource := ""
			if sourceTypeInfo != nil && sourceTypeInfo.LocalAlias != "" {
				localAliasForSource = sourceTypeInfo.LocalAlias
			} else if alias, ok := w.localTypeNameToFQN[typeSpec.Name.Name]; ok && alias == effectiveSourceType {
				localAliasForSource = typeSpec.Name.Name
			}

			// Create a new ConversionConfig, cloning from defaultConfig
			typeCfg := w.defaultConfig.Clone() // NEW: Clone defaultConfig
			typeCfg.Source = &types.EndpointConfig{
				Type:       effectiveSourceType,
				LocalAlias: localAliasForSource, // Set LocalAlias
			}

			w.parseAndApplyDirective(definingDirective, typeCfg) // pkgConfigs removed
			for _, mod := range modifierDirectives {
				w.parseAndApplyDirective(mod, typeCfg) // pkgConfigs removed
			}

			if typeCfg.Target.Type != "" { // This condition determines if the conversion is added
				w.AddConversion(typeCfg)
				slog.Debug("applyTypeLevelDirectives: Added type-level conversion config", "sourceType", typeCfg.Source.Type, "targetType", typeCfg.Target.Type)
			} else {
				slog.Debug("applyTypeLevelDirectives: Did not add type-level conversion config due to empty target type", "sourceType", typeCfg.Source.Type)
			}
		}
	}
}

// parseDirective is a centralized utility to parse a raw directive line.
// It returns the full key (e.g., "convert:target:suffix") and the cleaned-up value.
func parseDirective(line string) (key string, value string) {
	if !strings.HasPrefix(line, "//go:abgen:") {
		return "", ""
	}
	directive := strings.TrimPrefix(line, "//go:abgen:")

	parts := strings.SplitN(directive, "=", 2)
	key = parts[0]
	if len(parts) > 1 {
		valStr := parts[1]
		// Strip trailing comments before trimming quotes and spaces
		if commentIdx := strings.Index(valStr, "//"); commentIdx != -1 {
			valStr = valStr[:commentIdx]
		}
		value = strings.Trim(strings.TrimSpace(valStr), `"`)
	}
	return key, value
}

// parseAndApplyDirective parses a directive line and applies its settings to the given ConversionConfig.
// It no longer distinguishes between type-level and file-level processing via `typeCfg == nil`.
func (w *PackageWalker) parseAndApplyDirective(line string, cfg *types.ConversionConfig) {
	keyStr, value := parseDirective(line)
	if keyStr == "" {
		return
	}

	keys := strings.Split(keyStr, ":")
	verb := keys[0]
	subject := ""
	if len(keys) > 1 {
		subject = keys[1]
	}

	slog.Debug("parseAndApplyDirective: Processing directive", "line", line, "verb", verb, "subject", subject, "value", value)

	switch verb {
	case "package":
		if subject == "path" {
			// value will be "github.com/path,alias=ent"
			parts := strings.Split(value, ",")
			if len(parts) == 2 {
				pathPart := strings.TrimSpace(parts[0])
				aliasPart := strings.TrimSpace(parts[1])
				if strings.HasPrefix(aliasPart, "alias=") {
					alias := strings.TrimPrefix(aliasPart, "alias=")
					w.pathAliases[alias] = pathPart
					slog.Debug("parseAndApplyDirective: Registered package path alias", "alias", alias, "path", pathPart)
				}
			}
		}
	case "pair":
		if subject == "packages" {
			paths := strings.Split(value, ",")
			if len(paths) == 2 {
				// Pair packages directive now updates the SourcePackage and TargetPackage fields of the config
				// It doesn't create new ConversionConfigs here; that's handled in generateConversionFunctions.
				cfg.SourcePackage = strings.TrimSpace(paths[0])
				cfg.TargetPackage = strings.TrimSpace(paths[1])
				slog.Debug("parseAndApplyDirective: Set SourcePackage/TargetPackage", "source", cfg.SourcePackage, "target", cfg.TargetPackage)
			}
		}
	case "convert":
		switch len(keys) {
		case 1: // convert="A,B"
			if cfg.Source == nil {
				cfg.Source = &types.EndpointConfig{}
			}
			if cfg.Target == nil {
				cfg.Target = &types.EndpointConfig{}
			}
			parts := strings.Split(value, ",")
			if len(parts) == 2 {
				sourceName := strings.TrimSpace(parts[0])
				targetName := strings.TrimSpace(parts[1])

				sourceInfo := w.resolveTargetType(sourceName)
				if sourceInfo != nil && sourceInfo.Name != "" {
					cfg.Source.Type = sourceInfo.ImportPath + "." + sourceInfo.Name
					cfg.Source.LocalAlias = sourceInfo.LocalAlias
				} else {
					cfg.Source.Type = sourceName
				}

				targetInfo := w.resolveTargetType(targetName)
				if targetInfo != nil && targetInfo.Name != "" {
					cfg.Target.Type = targetInfo.ImportPath + "." + targetInfo.Name
					cfg.Target.LocalAlias = targetInfo.LocalAlias
				} else {
					cfg.Target.Type = targetName
				}
				slog.Debug("parseAndApplyDirective: Set convert types", "source", cfg.Source.Type, "target", cfg.Target.Type)
			}
		case 2: // Handles convert:direction, convert:ignore, convert:remap, convert:rule
			switch subject { // subject is keys[1]
			case "direction":
				cfg.Direction = value
			case "ignore":
				// Correctly handle "FQN#Field1,Field2,Field3" format
				var currentFQNPrefix string
				parts := strings.Split(value, ",")
				for _, part := range parts {
					trimmedPart := strings.TrimSpace(part)
					if strings.Contains(trimmedPart, "#") {
						// This part contains a full FQN, update the prefix
						prefixParts := strings.Split(trimmedPart, "#")
						currentFQNPrefix = prefixParts[0] + "#"
						cfg.IgnoreFields[trimmedPart] = true
					} else if currentFQNPrefix != "" {
						// This part is just a field name, prepend the last seen FQN prefix
						cfg.IgnoreFields[currentFQNPrefix+trimmedPart] = true
					} else {
						// This part has no FQN prefix, add it as is (might be less common)
						cfg.IgnoreFields[trimmedPart] = true
					}
				}
			case "remap":
				remapParts := strings.Split(value, ";")
				for _, remapEntry := range remapParts {
					kv := strings.SplitN(remapEntry, ":", 2)
					if len(kv) == 2 {
						cfg.RemapFields[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
					}
				}
			case "rule":
				rule := parseRule(value, w)
				if rule.SourceTypeName != "" && rule.TargetTypeName != "" && rule.ConvertFunc != "" {
					cfg.TypeConversionRules = append(cfg.TypeConversionRules, rule)
				}
			}
		case 3: // Handles convert:source:suffix, convert:target:prefix etc.
			subject, property := keys[1], keys[2]
			switch subject {
			case "source":
				if property == "suffix" {
					cfg.SourceSuffix = value
				} else if property == "prefix" {
					cfg.SourcePrefix = value
				}
			case "target":
				if property == "suffix" {
					cfg.TargetSuffix = value
				} else if property == "prefix" {
					cfg.TargetPrefix = value
				}
			}
		}
	}
}

// ProcessPackagePairs iterates through configs that have package pairs defined,
// finds matching types, and creates explicit conversion rules for them.
func (w *PackageWalker) ProcessPackagePairs() error {
	slog.Info("开始处理包配对")
	// Because w.rules can be modified during iteration, we create a snapshot of the keys first.
	var configKeys []string
	for k := range w.rules {
		configKeys = append(configKeys, k)
	}

	pairedConfigs := make(map[string]*types.ConversionConfig)

	// We only need to check the defaultConfig for the pair:packages directive.
	if w.defaultConfig.SourcePackage != "" && w.defaultConfig.TargetPackage != "" {
		key := w.defaultConfig.SourcePackage + "->" + w.defaultConfig.TargetPackage
		pairedConfigs[key] = w.defaultConfig
	}

	for _, cfg := range w.rules {
		if cfg.SourcePackage != "" && cfg.TargetPackage != "" {
			key := cfg.SourcePackage + "->" + cfg.TargetPackage
			// Store configs in a map to process each package pair only once.
			// The last config for a given pair will overwrite previous ones, which is acceptable
			// as they should ideally share the same package-level settings.
			pairedConfigs[key] = cfg
		}
	}

	for pairKey, baseCfg := range pairedConfigs {
		slog.Debug("处理配对", "key", pairKey)
		sourcePkgPath := baseCfg.SourcePackage
		targetPkgPath := baseCfg.TargetPackage

		// Resolve aliases
		if resolvedPath, ok := w.pathAliases[sourcePkgPath]; ok {
			sourcePkgPath = resolvedPath
		}
		if resolvedPath, ok := w.pathAliases[targetPkgPath]; ok {
			targetPkgPath = resolvedPath
		}

		// Load packages using the walker's loading mechanism
		sourcePkg, err := w.loadPackage(sourcePkgPath)
		if err != nil {
			return fmt.Errorf("failed to load source package %s for pairing: %w", sourcePkgPath, err)
		}
		targetPkg, err := w.loadPackage(targetPkgPath)
		if err != nil {
			return fmt.Errorf("failed to load target package %s for pairing: %w", targetPkgPath, err)
		}
		w.AddPackages(sourcePkg, targetPkg)

		// Get all type names from both packages
		sourceTypes := w.getPackageTypes(sourcePkg)
		targetTypes := w.getPackageTypes(targetPkg)

		// Find common type names
		for typeName := range sourceTypes {
			if _, exists := targetTypes[typeName]; exists {
				// Found a matching type by name
				sourceFQN := sourcePkg.PkgPath + "." + typeName
				targetFQN := targetPkg.PkgPath + "." + typeName

				// Check if a conversion rule already exists for this pair
				if _, exists := w.rules[sourceFQN+"2"+targetFQN]; exists {
					slog.Debug("已存在转换规则，跳过", "source", sourceFQN, "target", targetFQN)
					continue
				}

				slog.Info("发现配对类型，添加新的转换配置", "source", sourceFQN, "target", targetFQN)
				// Create a new specific conversion config, inheriting from the package-pair config
				newCfg := baseCfg.Clone()
				newCfg.Source = &types.EndpointConfig{Type: sourceFQN}
				newCfg.Target = &types.EndpointConfig{Type: targetFQN}

				w.AddConversion(newCfg)
			}
		}
	}
	slog.Info("包配对处理完成")
	return nil
}

// getPackageTypes extracts all exported type names from a loaded package's scope.
func (w *PackageWalker) getPackageTypes(pkg *packages.Package) map[string]bool {
	types := make(map[string]bool)
	if pkg == nil || pkg.Types == nil {
		return types
	}
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		if obj := scope.Lookup(name); obj != nil && obj.Exported() {
			// We only care about type names
			if _, ok := obj.(*gotypes.TypeName); ok {
				types[name] = true
			}
		}
	}
	return types
}

func parseRule(value string, walker *PackageWalker) types.TypeConversionRule {
	var rule types.TypeConversionRule
	ruleParts := strings.Split(value, ",")
	for _, part := range ruleParts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) == 2 {
			switch strings.TrimSpace(kv[0]) {
			case "source":
				rawTypeName := strings.TrimSpace(kv[1])
				slog.Debug("parseRule: Resolving source type", "rawTypeName", rawTypeName)
				resolvedType, err := walker.Resolve(rawTypeName)
				if err != nil {
					slog.Warn("Failed to resolve source type in rule", "type", rawTypeName, "error", err)
					rule.SourceTypeName = rawTypeName
				} else {
					if resolvedType.ImportPath == "builtin" {
						rule.SourceTypeName = resolvedType.Name
					} else {
						rule.SourceTypeName = resolvedType.ImportPath + "." + resolvedType.Name
					}
				}
			case "target":
				rawTypeName := strings.TrimSpace(kv[1])
				slog.Debug("parseRule: Resolving target type", "rawTypeName", rawTypeName)
				resolvedType, err := walker.Resolve(rawTypeName)
				if err != nil {
					slog.Warn("Failed to resolve target type in rule", "type", rawTypeName, "error", err)
					rule.TargetTypeName = rawTypeName
				} else {
					if resolvedType.ImportPath == "builtin" {
						rule.TargetTypeName = resolvedType.Name
					} else {
						rule.TargetTypeName = resolvedType.ImportPath + "." + resolvedType.Name
					}
				}
			case "func":
				rule.ConvertFunc = strings.TrimSpace(kv[1])
			}
		}
	}
	return rule
}

// resolveTargetType resolves the TypeInfo for a given type name, which may be in an external package.
func (w *PackageWalker) resolveTargetType(targetType string) *types.TypeInfo { // Changed return type to *types.TypeInfo
	slog.Debug("resolveTargetType: starting type resolution", "targetType", targetType)
	// Handle alias prefix e.g. "ent.User"
	parts := strings.Split(targetType, ".")
	if len(parts) == 2 {
		pkgAlias := parts[0]
		typeName := parts[1]
		if fullPath, ok := w.pathAliases[pkgAlias]; ok {
			targetType = fullPath + "." + typeName // Resolve to full path
			slog.Debug("resolveTargetType: resolved alias", "alias", pkgAlias, "fullType", targetType)
		}
	}
	// 0. Handle built-in types (e.g., "string", "int", "bool")
	// These types don't have an import path or package name in the same way as custom types.
	if gotypes.Universe.Lookup(targetType) != nil { // Check if it's a Go built-in type
		info := &types.TypeInfo{ // Store pointer
			Name:       targetType,
			PkgName:    "builtin",
			ImportPath: "builtin", // A special placeholder for built-in types
		}
		w.typeCache[targetType] = info
		slog.Debug("resolveTargetType: found built-in type", "targetType", targetType, "returnedInfo.Name", info.Name)
		return info
	}

	// 1. Check cache first
	if info, exists := w.typeCache[targetType]; exists {
		slog.Debug("resolveTargetType: found in cache", "targetType", targetType, "info.Name", info.Name, "info.ImportPath", info.ImportPath)
		return info // Return cached entry
	}

	// Immediately add a placeholder to the cache to prevent infinite recursion for cyclic type definitions
	// The actual content will be updated later.
	w.typeCache[targetType] = &types.TypeInfo{Name: targetType} // Store pointer
	slog.Debug("resolveTargetType: added placeholder to cache", "targetType", targetType)

	// 2. Handle local aliases within all known packages
	slog.Debug("resolveTargetType: checking local aliases in all known packages", "targetType", targetType)
	for _, pkg := range w.allKnownPkgs {
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				if genDecl, ok := decl.(*goast.GenDecl); ok && genDecl.Tok == token.TYPE {
					for _, spec := range genDecl.Specs {
						if typeSpec, ok := spec.(*goast.TypeSpec); ok {
							if typeSpec.Name.Name == targetType { // THIS IS WHERE THE COMPARISON HAPPENS
								slog.Debug("resolveTargetType: successfully found TypeSpec", "typeName", typeSpec.Name.Name, "pkgPath", pkg.PkgPath)

								// Call a unified helper, passing explicit package context
								info := w.buildTypeInfoFromTypeSpec(typeSpec, pkg)

								// If this TypeSpec is just an alias to another type, resolve the actual underlying type.
								// This logic remains similar but uses the IsAlias flag set by buildTypeInfoFromTypeSpec.
								if info.IsAlias && info.AliasFor != "" {
									underlyingTypeName := info.AliasFor // AliasFor contains the FQN of the underlying type
									slog.Debug("resolveTargetType: type is an alias, recursively resolving", "alias", targetType, "underlyingType", underlyingTypeName)
									// Resolve the underlying type.
									resolvedUnderlyingInfo := w.resolveTargetType(underlyingTypeName)
									if resolvedUnderlyingInfo != nil && resolvedUnderlyingInfo.Name != "" {
										// Transfer resolved info, but keep local alias specific info
										resolvedUnderlyingInfo.LocalAlias = info.LocalAlias
										resolvedUnderlyingInfo.IsAlias = true
										resolvedUnderlyingInfo.AliasFor = info.AliasFor
										info = *resolvedUnderlyingInfo // Dereference for value copy
									}
									// In this case, 'info' already contains minimal alias info from buildTypeInfoFromTypeSpec.
								}
								w.typeCache[targetType] = &info // Store pointer
								slog.Debug("resolveTargetType: returning TypeInfo", "typeName", info.Name, "pkgName", info.PkgName, "importPath", info.ImportPath)
								return &info // Return pointer
							}
						}
					}
				}
			}
		}
	}

	// 3. Handle qualified types (e.g., "ent.User" or "some/path/to/pkg.MyType")
	parts = strings.Split(targetType, ".")
	if len(parts) > 1 {
		pkgIdentifier := strings.Join(parts[:len(parts)-1], ".") // Handle full path like "a/b/c.Type"
		typeName := parts[len(parts)-1]
		slog.Debug("resolveTargetType: handling qualified type", "pkgIdentifier", pkgIdentifier, "typeName", typeName)

		var foundPkg *packages.Package

		// First, try to find the package by its full PkgPath if pkgIdentifier is a full path
		slog.Debug("resolveTargetType: trying to find by full package path", "pkgIdentifier", pkgIdentifier)
		for _, p := range w.allKnownPkgs {
			if p.PkgPath == pkgIdentifier {
				foundPkg = p
				slog.Debug("resolveTargetType: matched known package (by full path)", "foundPkg.PkgPath", foundPkg.PkgPath, "foundPkg.Name", foundPkg.Name)
				break
			}
			slog.Debug("resolveTargetType: pkg path mismatch", "p.PkgPath", p.PkgPath, "pkgIdentifier", pkgIdentifier)
		}

		// If not found by full PkgPath, try to resolve via imports (alias)
		if foundPkg == nil {
			slog.Debug("resolveTargetType: not found by full path, trying by import alias", "pkgIdentifier", pkgIdentifier)
			if importPath, exists := w.imports[pkgIdentifier]; exists {
				slog.Debug("resolveTargetType: found import alias", "alias", pkgIdentifier, "importPath", importPath)
				for _, p := range w.allKnownPkgs {
					if p.PkgPath == importPath {
						foundPkg = p
						slog.Debug("resolveTargetType: matched known package (by alias)", "foundPkg.PkgPath", foundPkg.PkgPath, "foundPkg.Name", foundPkg.Name)
						break
					}
				}
				if foundPkg == nil {
					slog.Debug("resolveTargetType: import alias exists, but corresponding package not found in w.allKnownPkgs", "importPath", importPath, "allKnownPkgs_paths", func() []string {
						paths := make([]string, len(w.allKnownPkgs))
						for i, p := range w.allKnownPkgs {
							paths[i] = p.PkgPath
						}
						return paths
					}())
				}
			}
		}

		if foundPkg != nil {
			slog.Debug("resolveTargetType: looking for type in found package", "foundPkg.PkgPath", foundPkg.PkgPath, "typeName", typeName)
			// 5. Find the type spec in the found package
			for _, file := range foundPkg.Syntax {
				slog.Debug("resolveTargetType: examining file in foundPkg.Syntax", "fileName", foundPkg.Fset.File(file.Pos()).Name(), "foundPkg.PkgPath", foundPkg.PkgPath)
				for _, decl := range file.Decls {
					if genDecl, ok := decl.(*goast.GenDecl); ok && genDecl.Tok == token.TYPE {
						for _, spec := range genDecl.Specs {
							if typeSpec, ok := spec.(*goast.TypeSpec); ok {
								slog.Debug("resolveTargetType: checking TypeSpec", "foundTypeSpecName", typeSpec.Name.Name, "lookingFor", typeName, "file", foundPkg.Fset.File(file.Pos()).Name())
								if typeSpec.Name.Name == typeName { // THIS IS WHERE THE COMPARISON HAPPENS
									slog.Debug("resolveTargetType: successfully found TypeSpec", "typeName", typeSpec.Name.Name, "pkgPath", foundPkg.PkgPath)

									info := w.buildTypeInfoFromTypeSpec(typeSpec, foundPkg)
									w.typeCache[targetType] = &info // Store pointer
									slog.Debug("resolveTargetType: returning TypeInfo", "typeName", info.Name, "pkgName", info.PkgName, "importPath", info.ImportPath)
									return &info // Return pointer
								}
							}
						}
					}
				}
			}
			slog.Debug("resolveTargetType: TypeSpec not found in found package's Syntax", "typeName", typeName, "foundPkg.PkgPath", foundPkg.PkgPath)
		} else {
			slog.Debug("resolveTargetType: qualified type's package not found in known packages, trying dynamic load", "pkgIdentifier", pkgIdentifier, "typeName", typeName)
			// Attempt dynamic package loading
			if strings.Contains(pkgIdentifier, "/") {
				slog.Debug("resolveTargetType: attempting dynamic package load", "pkgPath", pkgIdentifier)
				if loadedPkg, err := w.loadPackage(pkgIdentifier); err == nil {
					slog.Debug("resolveTargetType: successfully dynamically loaded package", "pkgPath", loadedPkg.PkgPath, "pkgName", loadedPkg.Name)
					foundPkg = loadedPkg
					w.allKnownPkgs = append(w.allKnownPkgs, loadedPkg)
					// Re-attempt resolution with the newly loaded package
					for _, file := range foundPkg.Syntax {
						for _, decl := range file.Decls {
							if genDecl, ok := decl.(*goast.GenDecl); ok && genDecl.Tok == token.TYPE {
								for _, spec := range genDecl.Specs {
									if typeSpec, ok := spec.(*goast.TypeSpec); ok && typeSpec.Name.Name == typeName {
										slog.Debug("resolveTargetType: found type in dynamically loaded package", "typeName", typeName)
										info := w.buildTypeInfoFromTypeSpec(typeSpec, foundPkg)
										w.typeCache[targetType] = &info
										slog.Debug("resolveTargetType: returning TypeInfo from dynamically loaded package", "typeName", info.Name, "pkgName", info.PkgName, "importPath", info.ImportPath)
										return &info
									}
								}
							}
						}
					}
				} else {
					slog.Debug("resolveTargetType: dynamic package load failed", "pkgPath", pkgIdentifier, "error", err)
				}
			}
		}
	}

	// If we reach here, the type was not found. Return an empty TypeInfo.
	slog.Debug("resolveTargetType: type not found, returning empty TypeInfo", "targetType", targetType)
	delete(w.typeCache, targetType) // Remove the placeholder
	return nil                      // Return nil for not found
}

// findAliasForPath finds the alias used for a given import path in the current context.
func (w *PackageWalker) findAliasForPath(importPath string) string {
	for alias, path := range w.imports {
		if path == importPath {
			// If the alias is the same as the last part of the path, it's an implicit alias.
			// In generated code, we might not need to specify it if it's not conflicting.
			// However, for explicit aliases (like `typespb`), we must use them.
			pathParts := strings.Split(path, "/")
			if alias != "." && alias != "_" && alias != pathParts[len(pathParts)-1] {
				return alias
			}
		}
	}
	// If no explicit alias is found, return the package's own name,
	// which is the default behavior of Go imports.
	if pkg, ok := w.loadedPkgs[importPath]; ok {
		return pkg.Name
	}
	// Fallback if package not loaded (should be rare)
	parts := strings.Split(importPath, "/")
	return parts[len(parts)-1]
}

// buildTypeInfoFromTypeSpec parses a given go/ast.TypeSpec or goast.Expr and returns a types.TypeInfo.
// It handles struct, pointer, slice, map, and basic type specifications.
func (w *PackageWalker) buildTypeInfoFromTypeSpec(spec interface{}, pkg *packages.Package) types.TypeInfo {
	var typeName string
	var expr goast.Expr

	switch s := spec.(type) {
	case *goast.TypeSpec:
		typeName = s.Name.Name
		expr = s.Type
	case goast.Expr:
		// This path is taken when resolving element types of slices/maps, or base type of pointers
		expr = s
		// Try to resolve a name for the expression if it's a simple identifier or selector
		if ident, ok := expr.(*goast.Ident); ok {
			typeName = ident.Name
		} else if sel, ok := expr.(*goast.SelectorExpr); ok {
			typeName = w.exprToString(sel, pkg) // Use exprToString to get qualified name
			if parts := strings.Split(typeName, "."); len(parts) > 1 {
				typeName = parts[len(parts)-1] // Just the base name
			}
		} else {
			typeName = w.exprToString(expr, pkg) // Fallback for complex expressions
		}
	default:
		return types.TypeInfo{} // Should not happen
	}

	info := types.TypeInfo{
		Name:        typeName,
		PkgName:     pkg.Name,
		ImportPath:  pkg.PkgPath,
		ImportAlias: w.findAliasForPath(pkg.PkgPath),
	}

	// Handle pointer types
	if starExpr, ok := expr.(*goast.StarExpr); ok {
		info.IsPointer = true
		// Recursively resolve the element type
		elemTypeInfo := w.buildTypeInfoFromTypeSpec(starExpr.X, pkg)
		info.ElemType = &elemTypeInfo
		return info
	}

	// Handle slice types
	if arrayType, ok := expr.(*goast.ArrayType); ok {
		info.IsSlice = true
		// Recursively resolve the element type
		elemTypeInfo := w.buildTypeInfoFromTypeSpec(arrayType.Elt, pkg)
		info.ElemType = &elemTypeInfo
		return info
	}

	// Handle map types
	if mapType, ok := expr.(*goast.MapType); ok {
		info.IsMap = true
		// Recursively resolve key and value types
		keyTypeInfo := w.buildTypeInfoFromTypeSpec(mapType.Key, pkg)
		info.KeyType = &keyTypeInfo
		valueTypeInfo := w.buildTypeInfoFromTypeSpec(mapType.Value, pkg)
		info.ValueType = &valueTypeInfo
		return info
	}

	if structType, ok := expr.(*goast.StructType); ok {
		info.Kind = types.TypeKindStruct
		for _, field := range structType.Fields.List {
			if len(field.Names) == 0 { // Embedded field
				// Recursively resolve embedded field's type and add its fields
				embeddedTypeInfo := w.buildTypeInfoFromTypeSpec(field.Type, pkg)
				info.Fields = append(info.Fields, embeddedTypeInfo.Fields...)
				continue
			}

			fieldName := field.Names[0].Name
			if goast.IsExported(fieldName) {
				fieldInfo := types.StructField{
					Name:     fieldName,
					Type:     w.exprToString(field.Type, pkg),
					Exported: true,
				}
				// Check if the field itself is a pointer
				if _, ok := field.Type.(*goast.StarExpr); ok {
					fieldInfo.IsPointer = true
				}
				info.Fields = append(info.Fields, fieldInfo)
			}
		}
	} else {
		// It's a named type but not a struct (e.g., `type Status string`), or an alias to another type.
		// If expr is an Ident or SelectorExpr, it's an alias or a basic type.
		if ident, ok := expr.(*goast.Ident); ok {
			underlyingTypeName := w.exprToString(ident, pkg)
			if underlyingTypeName != info.Name && !types.IsPrimitiveType(underlyingTypeName) {
				// This info is for the alias itself, not the underlying type's structure
				info.IsAlias = true
				info.AliasFor = underlyingTypeName
				// The Kind of the alias is the Kind of its underlying type
				underlyingInfo := w.resolveTargetType(underlyingTypeName)
				if underlyingInfo != nil {
					info.Kind = underlyingInfo.Kind
					info.IsPointer = underlyingInfo.IsPointer
					info.IsSlice = underlyingInfo.IsSlice
					info.IsMap = underlyingInfo.IsMap
					info.ElemType = underlyingInfo.ElemType
					info.KeyType = underlyingInfo.KeyType
					info.ValueType = underlyingInfo.ValueType
				}
			} else {
				info.Kind = types.TypeKindBasic
			}
		} else if selExpr, ok := expr.(*goast.SelectorExpr); ok {
			underlyingTypeName := w.exprToString(selExpr, pkg)
			if underlyingTypeName != info.Name && !types.IsPrimitiveType(underlyingTypeName) {
				info.IsAlias = true
				info.AliasFor = underlyingTypeName
				// The Kind of the alias is the Kind of its underlying type
				underlyingInfo := w.resolveTargetType(underlyingTypeName)
				if underlyingInfo != nil {
					info.Kind = underlyingInfo.Kind
					info.IsPointer = underlyingInfo.IsPointer
					info.IsSlice = underlyingInfo.IsSlice
					info.IsMap = underlyingInfo.IsMap
					info.ElemType = underlyingInfo.ElemType
					info.KeyType = underlyingInfo.KeyType
					info.ValueType = underlyingInfo.ValueType
				}
			} else {
				info.Kind = types.TypeKindBasic
			}
		} else {
			info.Kind = types.TypeKindBasic // Default for other expressions
		}
	}
	return info
}

// IsStructOrStructAlias checks if the given type spec is a struct or an alias to a struct.
func (w *PackageWalker) IsStructOrStructAlias(typeSpec *goast.TypeSpec) bool {
	if _, ok := typeSpec.Type.(*goast.StructType); ok {
		return true
	}
	if ident, ok := typeSpec.Type.(*goast.Ident); ok {
		targetInfo := w.resolveTargetType(ident.Name)
		return targetInfo != nil && targetInfo.Kind == types.TypeKindStruct
	}
	if selector, ok := typeSpec.Type.(*goast.SelectorExpr); ok {
		typeName := w.exprToString(selector, w.currentPkg)
		targetInfo := w.resolveTargetType(typeName)
		return targetInfo != nil && targetInfo.Kind == types.TypeKindStruct
	}
	return false
}

// loadPackage loads a package by its import path, using a cache to avoid redundant loads.
func (w *PackageWalker) loadPackage(importPath string) (*packages.Package, error) {
	if pkg, ok := w.loadedPkgs[importPath]; ok {
		return pkg, nil
	}
	cfg := &packages.Config{Mode: w.packageMode}
	pkgs, err := packages.Load(cfg, importPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load package %s: %w", importPath, err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no package found for import path %s", importPath)
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("errors while loading package %s", importPath)
	}
	w.loadedPkgs[importPath] = pkgs[0]
	w.AddPackages(pkgs[0])
	return pkgs[0], nil
}

// collectImports collects import statements from a slice of go/ast.File and populates the walker's imports map.
func (w *PackageWalker) collectImports(files []*goast.File) {
	for _, file := range files {
		for _, imp := range file.Imports {
			alias := ""
			if imp.Name != nil {
				alias = imp.Name.Name
			} else {
				parts := strings.Split(strings.Trim(imp.Path.Value, `"`), "/")
				alias = parts[len(parts)-1]
			}
			realPath := strings.Trim(imp.Path.Value, `"`)
			// Only add if not already present to avoid overwriting explicit aliases with implicit ones
			if _, exists := w.imports[alias]; !exists {
				w.imports[alias] = realPath
			}
		}
	}
}

// GetKnownTypes returns the cache of known types.
func (w *PackageWalker) GetKnownTypes() map[string]*types.TypeInfo { // Changed return type
	return w.typeCache
}

// GetRules returns the map of conversion configurations managed by the walker.
func (w *PackageWalker) GetRules() map[string]*types.ConversionConfig {
	return w.rules
}

// AddConversion adds a new ConversionConfig to the rules map, handling bidirectional conversions.
func (w *PackageWalker) AddConversion(cfg *types.ConversionConfig) {
	if w.rules == nil {
		return // If rules map is nil, we cannot add conversions.
	}

	configKey := cfg.Source.Type + "2" + cfg.Target.Type
	w.rules[configKey] = cfg

	if cfg.Direction == "both" {
		reverseCfg := cfg.Clone() // Clone to create a mutable copy for reverse
		// Swap source and target
		reverseCfg.Source = cfg.Target.Clone()
		reverseCfg.Target = cfg.Source.Clone()
		// Swap prefixes/suffixes for the reverse conversion as well
		reverseCfg.SourcePrefix = cfg.TargetPrefix
		reverseCfg.SourceSuffix = cfg.TargetSuffix
		reverseCfg.TargetPrefix = cfg.SourcePrefix
		reverseCfg.TargetSuffix = cfg.SourceSuffix
		reverseCfg.Direction = "to" // The reverse is always a simple "to"

		w.AddConversion(reverseCfg)
	}
}
