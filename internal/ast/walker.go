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
	imports map[string]string

	rules map[string]*types.ConversionConfig // The global rule set, key: SourceFQN->TargetFQN

	currentPkg *packages.Package

	typeCache map[string]types.TypeInfo

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
		for _, knownPkg := range w.allKnownPkgs {
			if knownPkg.Types.Path() == p.Path() {
				return knownPkg.PkgPath
			}
		}
		return p.Path()
	}

	return gotypes.TypeString(tv.Type, qualifier)
}

func (w *PackageWalker) GetTypeCache() map[string]types.TypeInfo {

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

		imports: make(map[string]string),

		typeCache: make(map[string]types.TypeInfo),

		loadedPkgs: make(map[string]*packages.Package),

		packageMode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports,

		defaultConfig: &types.ConversionConfig{ // NEW: Initialize default config
			Direction:    "both",
			IgnoreFields: make(map[string]bool),
			IgnoreTypes:  make(map[string]bool),
			RemapFields:  make(map[string]string),
			TypeConversionRules: make([]types.TypeConversionRule, 0),
		},

		localTypeNameToFQN: make(map[string]string),
	}
}

// Resolve is a public method, for resolving type information

func (w *PackageWalker) Resolve(typeName string) (types.TypeInfo, error) {

	typeName = strings.TrimPrefix(typeName, "*")

	info := w.resolveTargetType(typeName)

	if info.Name == "" {

		return types.TypeInfo{}, fmt.Errorf("type %q not found", typeName)

	}

	return info, nil

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
				if !strings.HasPrefix(line.Text, "//go:abgen:pair:packages=") {
					continue
				}

				directive := strings.TrimPrefix(line.Text, "//go:abgen:")
				parts := strings.SplitN(directive, "=", 2)
				if len(parts) != 2 {
					continue
				}

				valueStr := parts[1]
				var value string
				if strings.HasPrefix(valueStr, `"`) {
					lastQuoteIndex := strings.LastIndex(valueStr, `"`)
					if lastQuoteIndex > 0 {
						value = valueStr[1:lastQuoteIndex]
					}
				}

				if value != "" {
					paths := strings.Split(value, ",")
					for _, p := range paths {
						discoveredPaths[strings.TrimSpace(p)] = struct{}{}
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

// Analyze performs the main analysis pass. It assumes all necessary packages
// (the directive package and any discovered dependencies) have already been
// added to the walker via AddPackages. It then processes all directives
// and builds the conversion configurations.
// This function replaces the monolithic WalkPackage.
func (w *PackageWalker) WalkPackage(pkg *packages.Package) error {
	slog.Info("开始遍历包", "包", pkg.PkgPath)

	w.currentPkg = pkg

	w.collectImports(pkg.Syntax) // Collect imports from all files in the package

	slog.Debug("WalkPackage: collected imports", "imports", w.imports)

	// Add the current package and all its imported packages to allKnownPkgs

	w.AddPackages(pkg)

	for _, impPkg := range pkg.Imports {

		w.AddPackages(impPkg)

	}

	slog.Debug("WalkPackage: allKnownPkgs after adding imports", "allKnownPkgs", func() []string {

		paths := make([]string, len(w.allKnownPkgs))

		for i, p := range w.allKnownPkgs {

			paths[i] = p.PkgPath

		}

		return paths

	}())

	slog.Debug("WalkPackage: Number of files in pkg.Syntax", "count", len(pkg.Syntax))

	for _, file := range pkg.Syntax {

		filename := pkg.Fset.File(file.Pos()).Name()

		slog.Info("遍历文件", "文件", filename)

		if strings.HasSuffix(filepath.Base(filename), ".gen.go") {

			slog.Debug("WalkPackage: Skipping generated file", "file", filename)

			continue

		}

		if err := w.processFileDecls(file); err != nil {

			return err

		}

	}

	slog.Debug("WalkPackage: Finished processing package", "pkgPath", pkg.PkgPath)

	return nil

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
					if _, isStruct := typeSpec.Type.(*goast.StructType); isStruct {
						fqn = w.currentPkg.PkgPath + "." + typeSpec.Name.Name
					} else {
						underlyingTypeStr := w.exprToString(typeSpec.Type, w.currentPkg)
						resolvedInfo := w.resolveTargetType(underlyingTypeStr)
						if resolvedInfo.Name != "" {
							if resolvedInfo.ImportPath == "builtin" {
								fqn = resolvedInfo.Name
							} else {
								fqn = resolvedInfo.ImportPath + "." + resolvedInfo.Name
							}
						} else {
							fqn = underlyingTypeStr
						}
					}
					w.localTypeNameToFQN[typeSpec.Name.Name] = fqn
					// Create a TypeInfo entry for the local alias itself
					w.typeCache[typeSpec.Name.Name] = types.TypeInfo{
						Name:       typeSpec.Name.Name,
						PkgName:    w.currentPkg.Name,
						ImportPath: w.currentPkg.PkgPath,
						LocalAlias: typeSpec.Name.Name, // It's its own local alias
						IsAlias:    true,
						AliasFor:   fqn, // The FQN of the underlying type
					}
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
		isConvertDirective := false // Flag for convert="A,B"

		for _, comment := range commentGroup.List {
			line := comment.Text
			if !strings.HasPrefix(line, "//go:abgen:") {
				continue
			}
			isDirectiveGroup = true
			directive := strings.TrimPrefix(line, "//go:abgen:")

			if strings.HasPrefix(directive, "pair:packages=") {
				definingDirective = line
			} else if strings.HasPrefix(directive, "convert=") {
				definingDirective = line
				isConvertDirective = true
			} else { // This is for modifiers (like convert:direction, convert:remap)
				modifierDirectives = append(modifierDirectives, line)
			}
		}

		if !isDirectiveGroup {
			continue
		}

		// Try to find an associated type declaration for this comment group
		var associatedDecl *goast.GenDecl
		for _, decl := range file.Decls {
			if genDecl, ok := decl.(*goast.GenDecl); ok && genDecl.Tok == token.TYPE {
				// Case 1: Comment is directly above the GenDecl (type declaration)
				if genDecl.Doc != nil {
					if genDecl.Doc.Pos() == commentGroup.Pos() {
						associatedDecl = genDecl
						break
					}
				}

				// Case 2: Comment is associated with a TypeSpec within a GenDecl.
				// This handles `type ( // Comment A int )`
				if genDecl.Specs != nil { // Check if Specs is not nil
					for _, spec := range genDecl.Specs {
						if typeSpec, typeSpecOk := spec.(*goast.TypeSpec); typeSpecOk {
							if typeSpec.Doc != nil {
								if typeSpec.Doc.Pos() == commentGroup.Pos() {
									associatedDecl = genDecl // The GenDecl is the "associated declaration" for its specs
									break
								}
							}
						}
					}
				}
				if associatedDecl != nil { // Break outer loop if already found from specs
					break
				}
			}
		}

		if associatedDecl == nil { // If no associated type declaration
			// It's a file-level directive.
			// If it's a file-level convert="A,B" directive, we need to create a ConversionConfig for it.
			if isConvertDirective {
				typeCfg := w.defaultConfig.Clone() // NEW: Clone defaultConfig for this specific conversion
				
				// The source and target type names are directly in the value part of the definingDirective
				parts := strings.Split(strings.TrimPrefix(definingDirective, "//go:abgen:convert="), ",")
				if len(parts) == 2 {
					sourceName := strings.TrimSpace(parts[0])
					targetName := strings.TrimSpace(parts[1])

					sourceInfo := w.resolveTargetType(sourceName)
					if sourceInfo.Name != "" {
						typeCfg.Source.Type = sourceInfo.ImportPath + "." + sourceInfo.Name
						typeCfg.Source.LocalAlias = sourceInfo.LocalAlias
					} else {
						typeCfg.Source.Type = sourceName
					}

					targetInfo := w.resolveTargetType(targetName)
					if targetInfo.Name != "" {
						typeCfg.Target.Type = targetInfo.ImportPath + "." + targetInfo.Name
						typeCfg.Target.LocalAlias = targetInfo.LocalAlias
					} else {
						typeCfg.Target.Type = targetName
					}
				}

				// Apply any modifiers to this typeCfg
				for _, mod := range modifierDirectives {
					w.parseAndApplyDirective(mod, typeCfg) // pkgConfigs removed
				}
				if typeCfg.Source.Type != "" && typeCfg.Target.Type != "" { // Ensure both source and target are set
					w.AddConversion(typeCfg)
					slog.Debug("processCommentDirectives: Added file-level conversion config", "sourceType", typeCfg.Source.Type, "targetType", typeCfg.Target.Type)
				} else {
					slog.Debug("processCommentDirectives: Did not add file-level conversion config due to incomplete types", "definingDirective", definingDirective)
				}
			} else { // Other file-level directives (pair:packages, or modifiers without definingDirective)
				// Apply file-level directives to the defaultConfig
				w.applyFileLevelDirectives(definingDirective, modifierDirectives)
			}
		} else { // Type-level comment group (associated with a type declaration)
			w.applyTypeLevelDirectives(file, commentGroup, definingDirective, modifierDirectives, associatedDecl)
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
			effectiveSourceType := sourceTypeInfo.ImportPath + "." + sourceTypeInfo.Name
			localAliasForSource := ""
			if sourceTypeInfo.LocalAlias != "" {
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

// parseAndApplyDirective parses a directive line and applies its settings to the given ConversionConfig.
// It no longer distinguishes between type-level and file-level processing via `typeCfg == nil`.
func (w *PackageWalker) parseAndApplyDirective(line string, cfg *types.ConversionConfig) { // pkgConfigs removed
	directive := strings.TrimPrefix(line, "//go:abgen:")

	// Use SplitN to correctly handle '=' within the value part.
	parts := strings.SplitN(directive, "=", 2)
	keyStr := parts[0]
	value := ""
	if len(parts) > 1 {
		valStr := parts[1]
		// Correctly extract the value within quotes, ignoring trailing comments.
		if strings.HasPrefix(valStr, `"`) {
			// Find the closing quote.
			lastQuoteIndex := strings.LastIndex(valStr, `"`)
			if lastQuoteIndex > 0 { // Ensure it's not the opening quote
				value = valStr[1:lastQuoteIndex]
			}
		} else {
			// Handle cases where value is not quoted (if any)
			value = valStr
		}
	}
	
	keys := strings.Split(keyStr, ":")
	verb := keys[0]
	subject := ""
	if len(keys) > 1 {
		subject = keys[1]
	}

	slog.Debug("parseAndApplyDirective: Processing directive", "line", line, "verb", verb, "subject", subject, "value", value)

	switch verb {
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
			parts := strings.Split(value, ",")
			if len(parts) == 2 {
				sourceName := strings.TrimSpace(parts[0])
				targetName := strings.TrimSpace(parts[1])

				sourceInfo := w.resolveTargetType(sourceName)
				if sourceInfo.Name != "" {
					cfg.Source.Type = sourceInfo.ImportPath + "." + sourceInfo.Name
					cfg.Source.LocalAlias = sourceInfo.LocalAlias
				} else {
					cfg.Source.Type = sourceName
				}

				targetInfo := w.resolveTargetType(targetName)
				if targetInfo.Name != "" {
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
func (w *PackageWalker) resolveTargetType(targetType string) types.TypeInfo {
	slog.Debug("resolveTargetType: starting type resolution", "targetType", targetType)

	// 0. Handle built-in types (e.g., "string", "int", "bool")
	// These types don't have an import path or package name in the same way as custom types.
	if gotypes.Universe.Lookup(targetType) != nil { // Check if it's a Go built-in type
		info := types.TypeInfo{
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
	w.typeCache[targetType] = types.TypeInfo{Name: targetType}
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
									if resolvedUnderlyingInfo.Name != "" {
										// Transfer resolved info, but keep local alias specific info
										resolvedUnderlyingInfo.LocalAlias = info.LocalAlias
										resolvedUnderlyingInfo.IsAlias = true
										resolvedUnderlyingInfo.AliasFor = info.AliasFor
										info = resolvedUnderlyingInfo
									}
									// If resolvedUnderlyingInfo.Name is empty, it means the underlying type could not be resolved.
									// In this case, 'info' already contains minimal alias info from buildTypeInfoFromTypeSpec.
								}
								w.typeCache[targetType] = info // Cache the result with original targetType
								slog.Debug("resolveTargetType: returning TypeInfo", "typeName", info.Name, "pkgName", info.PkgName, "importPath", info.ImportPath)
								return info
							}
						}
					}
				}
			}
		}
	}

	// 3. Handle qualified types (e.g., "ent.User" or "some/path/to/pkg.MyType")
	parts := strings.Split(targetType, ".")
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
									w.typeCache[targetType] = info // Cache the result with original targetType
									slog.Debug("resolveTargetType: returning TypeInfo", "typeName", info.Name, "pkgName", info.PkgName, "importPath", info.ImportPath)
									return info
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
										w.typeCache[targetType] = info
										slog.Debug("resolveTargetType: returning TypeInfo from dynamically loaded package", "typeName", info.Name, "pkgName", info.PkgName, "importPath", info.ImportPath)
										return info
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
	return types.TypeInfo{}
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

// buildTypeInfoFromTypeSpec parses a given go/ast.TypeSpec and returns a types.TypeInfo.
// It handles both struct and non-struct type specifications.
func (w *PackageWalker) buildTypeInfoFromTypeSpec(typeSpec *goast.TypeSpec, pkg *packages.Package) types.TypeInfo {
	info := types.TypeInfo{
		Name:        typeSpec.Name.Name,
		PkgName:     pkg.Name,
		ImportPath:  pkg.PkgPath,
		ImportAlias: w.findAliasForPath(pkg.PkgPath),
	}

	if structType, ok := typeSpec.Type.(*goast.StructType); ok {
		// It's a struct, parse its fields
		info.Kind = types.TypeKindStruct
		for _, field := range structType.Fields.List {
			if len(field.Names) == 0 { // Embedded field
				var embeddedTypeExpr goast.Expr
				if ident, ok := field.Type.(*goast.Ident); ok {
					embeddedTypeExpr = ident
				} else if selExpr, ok := field.Type.(*goast.SelectorExpr); ok {
					embeddedTypeExpr = selExpr
				}
				if embeddedTypeExpr != nil {
					embeddedTypeName := w.exprToString(embeddedTypeExpr, pkg)
					embeddedInfo := w.resolveTargetType(embeddedTypeName)
					info.Fields = append(info.Fields, embeddedInfo.Fields...)
				}
				continue
			}

			fieldName := field.Names[0].Name
			if goast.IsExported(fieldName) {
				info.Fields = append(info.Fields, types.StructField{
					Name:     fieldName,
					Type:     w.exprToString(field.Type, pkg),
					Exported: true,
				})
			}
		}
	} else {
		// It's a named type but not a struct (e.g., `type Status string`), or an alias to another type.
		// Check if it's an alias to a non-struct type.
		// If typeSpec.Type is an Ident or SelectorExpr, it's an alias.
		if ident, ok := typeSpec.Type.(*goast.Ident); ok {
			underlyingTypeName := w.exprToString(ident, pkg) // Use pkg context for exprToString
			if underlyingTypeName != typeSpec.Name.Name && !types.IsPrimitiveType(underlyingTypeName) {
				info.IsAlias = true
				info.AliasFor = underlyingTypeName
			}
		} else if selExpr, ok := typeSpec.Type.(*goast.SelectorExpr); ok {
			underlyingTypeName := w.exprToString(selExpr, pkg) // Use pkg context for exprToString
			if underlyingTypeName != typeSpec.Name.Name && !types.IsPrimitiveType(underlyingTypeName) {
				info.IsAlias = true
				info.AliasFor = underlyingTypeName
			}
		}
		// Determine TypeKind for non-structs if needed
		// For now, assume if not a struct, it's basic or complex based on underlyingTypeStr
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
		return len(targetInfo.Fields) > 0
	}
	if selector, ok := typeSpec.Type.(*goast.SelectorExpr); ok {
		typeName := w.exprToString(selector, w.currentPkg)
		targetInfo := w.resolveTargetType(typeName)
		return len(targetInfo.Fields) > 0
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
	w.loadedPkgs[importPath] = pkgs[0]
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
func (w *PackageWalker) GetKnownTypes() map[string]types.TypeInfo {
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
		// Set direction for the reverse config
		reverseCfg.Direction = "to" // The reverse is always a simple "to"
		
		reverseKey := reverseCfg.Source.Type + "2" + reverseCfg.Target.Type
		w.rules[reverseKey] = reverseCfg
	}
}