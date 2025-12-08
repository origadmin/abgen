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

	graph types.ConversionGraph

	currentPkg *packages.Package

	typeCache map[string]types.TypeInfo

	loadedPkgs map[string]*packages.Package // 缓存已加载的包

	packageMode packages.LoadMode

	PackageConfigs []*types.PackageConversionConfig

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

// NewPackageWalker 创建新的遍历器

func NewPackageWalker(graph types.ConversionGraph) *PackageWalker {

	return &PackageWalker{

		graph: graph,

		imports: make(map[string]string),

		typeCache: make(map[string]types.TypeInfo),

		loadedPkgs: make(map[string]*packages.Package),

		packageMode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports,

		PackageConfigs: make([]*types.PackageConversionConfig, 0),

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

// WalkPackage 遍历包内的类型定义

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

	slog.Debug("WalkPackage: Finished processing package", "pkgPath", pkg.PkgPath, "PackageConfigsFound", len(w.PackageConfigs))

	return nil

}

func (w *PackageWalker) processFileDecls(file *goast.File) error {

	slog.Debug("processFileDecls: Processing file", "filename", w.currentPkg.Fset.File(file.Pos()).Name())

	// Pass 1: Collect all local type specs and resolve their FQNs.

	for _, decl := range file.Decls {

		if genDecl, ok := decl.(*goast.GenDecl); ok && genDecl.Tok == token.TYPE {

			for _, spec := range genDecl.Specs {

				if typeSpec, ok := spec.(*goast.TypeSpec); ok {

					// For any type `type T = some.OtherType` or `type T struct{}`,

					// we want to know the FQN of what `T` represents.

					var fqn string

					if _, isStruct := typeSpec.Type.(*goast.StructType); isStruct {

						// If it's a struct defined locally, its FQN is in the current package.

						fqn = w.currentPkg.PkgPath + "." + typeSpec.Name.Name

					} else {

						// It's an alias or a named primitive type. Resolve what it points to.

						underlyingTypeStr := w.exprToString(typeSpec.Type, w.currentPkg)

						resolvedInfo := w.resolveTargetType(underlyingTypeStr)

						if resolvedInfo.Name != "" {

							if resolvedInfo.ImportPath == "builtin" {

								fqn = resolvedInfo.Name // e.g., "string"

							} else {

								fqn = resolvedInfo.ImportPath + "." + resolvedInfo.Name

							}

						} else {

							// Fallback if resolution fails

							fqn = underlyingTypeStr

						}

					}

					w.localTypeNameToFQN[typeSpec.Name.Name] = fqn

					slog.Debug("processFileDecls: Collected local type", "localName", typeSpec.Name.Name, "fqn", fqn)

				}

			}

		}

	}

	// Pass 2: Find directive groups and process them.

	slog.Debug("processFileDecls: Number of comment groups", "count", len(file.Comments))

	for _, commentGroup := range file.Comments {

		slog.Debug("processFileDecls: Processing comment group", "commentGroup", commentGroup.Text())

		var definingDirective string

		var modifierDirectives []string

		isPackagePair := false

		isDirectiveGroup := false

		for _, comment := range commentGroup.List {

			line := comment.Text

			slog.Debug("processFileDecls: Processing comment line", "line", line)

			if !strings.HasPrefix(line, "//go:abgen:") {

				continue

			}

			isDirectiveGroup = true

			directive := strings.TrimPrefix(line, "//go:abgen:")

			slog.Debug("processFileDecls: Detected abgen directive", "directive", directive)

			if strings.HasPrefix(directive, "pair:packages=") {

				definingDirective = line

				isPackagePair = true

				slog.Debug("processFileDecls: Detected package pair directive", "definingDirective", definingDirective)

			} else if strings.HasPrefix(directive, "convert=") {

				definingDirective = line

				slog.Debug("processFileDecls: Detected convert directive", "definingDirective", definingDirective)

			} else {

				modifierDirectives = append(modifierDirectives, line)

				slog.Debug("processFileDecls: Detected modifier directive", "modifierDirective", line)

			}

		}

		if !isDirectiveGroup {
			slog.Debug("processFileDecls: Not a directive group or no defining directive found", "isDirectiveGroup", isDirectiveGroup, "definingDirective", definingDirective)

			continue

		}

		// This logic now handles any file-level directive group.
		// It applies all found directives to the single, package-level config.
		if isPackagePair || (definingDirective == "" && len(modifierDirectives) > 0) {
			if definingDirective != "" {
				w.parseAndApplyDirective(definingDirective, nil, map[string]*types.PackageConversionConfig{"pkg-pair": w.PackageConfigs[0]})
			}
			for _, mod := range modifierDirectives {
				w.parseAndApplyDirective(mod, nil, map[string]*types.PackageConversionConfig{"pkg-pair": w.PackageConfigs[0]})
			}
		} else { // This is a type-level convert config

			var associatedDecl *goast.GenDecl

			for _, decl := range file.Decls {

				genDecl, ok := decl.(*goast.GenDecl)

				if !ok || genDecl.Tok != token.TYPE {

					continue

				}

				doc := genDecl.Doc

				if doc == nil && len(genDecl.Specs) > 0 {

					if spec, ok := genDecl.Specs[0].(*goast.TypeSpec); ok {

						doc = spec.Doc

					}

				}

				if doc != nil && doc.Pos() == commentGroup.Pos() {

					associatedDecl = genDecl

					break

				}

			}

			if associatedDecl == nil {

				slog.Debug("processFileDecls: Type-level directive, but no associated declaration found")

				continue

			}

			for _, spec := range associatedDecl.Specs {

				typeSpec, ok := spec.(*goast.TypeSpec)

				if !ok {

					continue

				}

				// Here, we need to resolve the typeSpec's underlying type to get the FQN

				sourceTypeInfo := w.resolveTargetType(w.exprToString(typeSpec.Type, w.currentPkg))

				effectiveSourceType := sourceTypeInfo.ImportPath + "." + sourceTypeInfo.Name

				typeCfg := &types.ConversionConfig{

					Source: &types.EndpointConfig{Type: effectiveSourceType},

					Target: &types.EndpointConfig{},

					IgnoreFields: make(map[string]bool),

					RemapFields: make(map[string]string),

					TypeConversionRules: make([]types.TypeConversionRule, 0),
				}

				w.parseAndApplyDirective(definingDirective, typeCfg, nil)

				for _, mod := range modifierDirectives {

					w.parseAndApplyDirective(mod, typeCfg, nil)

				}

				if typeCfg.Target.Type != "" {

					w.AddConversion(typeCfg)

					slog.Debug("processFileDecls: Added type-level conversion config", "sourceType", typeCfg.Source.Type, "targetType", typeCfg.Target.Type)

				} else {

					slog.Debug("processFileDecls: Did not add type-level conversion config due to empty target type", "sourceType", typeCfg.Source.Type)

				}

			}

		}

	}

	return nil

}

func (w *PackageWalker) parseAndApplyDirective(line string, typeCfg *types.ConversionConfig, pkgConfigs map[string]*types.PackageConversionConfig) {
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

	if typeCfg == nil { // File-level
		const pkgConfigKey = "pkg-pair"
		if pkgConfigs == nil || pkgConfigs[pkgConfigKey] == nil {
			return
		}
		cfg := pkgConfigs[pkgConfigKey]
		switch verb {
		case "pair":
			if subject == "packages" {
				paths := strings.Split(value, ",")
				if len(paths) == 2 {
					cfg.SourcePackage = strings.TrimSpace(paths[0])
					cfg.TargetPackage = strings.TrimSpace(paths[1])
					slog.Debug("parseAndApplyDirective: Set pkg-pair SourcePackage/TargetPackage", "source", cfg.SourcePackage, "target", cfg.TargetPackage)
				}
			}
		case "convert":
			switch len(keys) {
			case 2: // Handles convert:direction, convert:ignore, convert:remap, convert:rule
				switch subject { // subject is keys[1]
				case "direction":
					cfg.Direction = value
				case "ignore":
					ignoreParts := strings.Split(value, ",")
					for _, part := range ignoreParts {
						cfg.IgnoreFields[strings.TrimSpace(part)] = true
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
		return
	}

	// Type-level
	if verb == "convert" {
		if len(keys) == 1 { // convert="A,B"
			parts := strings.Split(value, ",")
			if len(parts) == 2 {
				targetName := strings.TrimSpace(parts[1])
				// Resolve the target type using the proper function, which can handle local aliases.
				targetInfo := w.resolveTargetType(targetName)
				if targetInfo.Name != "" {
					typeCfg.Target.Type = targetInfo.ImportPath + "." + targetInfo.Name
				} else {
					// Fallback if resolution fails, though it should be rare.
					typeCfg.Target.Type = targetName
				}
				slog.Debug("parseAndApplyDirective: Set type-level Target.Type", "target", typeCfg.Target.Type)
			}
		} else if len(keys) == 2 { // convert:ignore="..."
			switch keys[1] {
			case "direction":
				typeCfg.Direction = value
			case "ignore":
				// Parse FQN#FieldName,FieldName,...
				ignoreParts := strings.Split(value, ",")
				for _, part := range ignoreParts {
					typeCfg.IgnoreFields[strings.TrimSpace(part)] = true
				}
			case "remap": // Type-level remap
				// Parse FQN#SourceFieldPath:TargetFieldPath
				remapParts := strings.Split(value, ";") // Allow multiple remaps separated by semicolon
				for _, remapEntry := range remapParts {
					kv := strings.SplitN(remapEntry, ":", 2)
					if len(kv) == 2 {
						typeCfg.RemapFields[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
					}
				}
			case "rule":
				rule := parseRule(value, w)
				if rule.SourceTypeName != "" && rule.TargetTypeName != "" && rule.ConvertFunc != "" {
					typeCfg.TypeConversionRules = append(typeCfg.TypeConversionRules, rule)
				}
			}
		} else if len(keys) == 3 {
			subject, property := keys[1], keys[2]
			switch subject {
			case "source":
				if property == "suffix" {
					typeCfg.Source.Suffix = value
				} else if property == "prefix" {
					typeCfg.Source.Prefix = value
				}
			case "target":
				if property == "suffix" {
					typeCfg.Target.Suffix = value
				} else if property == "prefix" {
					typeCfg.Target.Prefix = value
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
								// If this TypeSpec is just an alias to another type, resolve the actual type.
								if _, isStruct := typeSpec.Type.(*goast.StructType); !isStruct {
									underlyingTypeName := w.exprToString(typeSpec.Type, w.currentPkg)
									slog.Debug("resolveTargetType: type is an alias, recursively resolving", "alias", targetType, "underlyingType", underlyingTypeName)
									// Resolve the underlying type.
									resolvedUnderlyingInfo := w.resolveTargetType(underlyingTypeName)
									if resolvedUnderlyingInfo.Name != "" {
										// Update the cache for the ALIAS name to the resolved underlying type info.
										w.typeCache[targetType] = resolvedUnderlyingInfo
										slog.Debug("resolveTargetType: returning resolved underlying type for alias", "alias", targetType, "underlyingTypeInfo.Name", resolvedUnderlyingInfo.Name)
										return resolvedUnderlyingInfo
									}
								}

								// It's a struct definition or a non-struct type whose underlying type is not a struct.
								// Parse its fields or create TypeInfo.
								var info types.TypeInfo
								if _, isStruct := typeSpec.Type.(*goast.StructType); isStruct {
									info = w.parseStructFields(typeSpec, pkg)
								} else {
									info = types.TypeInfo{
										Name:        typeSpec.Name.Name,
										PkgName:     pkg.Name,
										ImportPath:  pkg.PkgPath,
										ImportAlias: w.findAliasForPath(pkg.PkgPath),
									}
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
									var info types.TypeInfo
									// Check if it's a struct type
									if _, isStruct := typeSpec.Type.(*goast.StructType); isStruct {
										// If it's a struct, parse its fields
										info = w.parseStructFields(typeSpec, foundPkg)
									} else {
										// If it's a named type but not a struct (e.g., `type Status string`),
										// just create a TypeInfo without parsing fields.
										info = types.TypeInfo{
											Name:        typeSpec.Name.Name,
											PkgName:     foundPkg.Name,
											ImportPath:  foundPkg.PkgPath,
											ImportAlias: w.findAliasForPath(foundPkg.PkgPath),
											// Fields remains empty for non-struct types
										}
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
										info := w.parseStructFields(typeSpec, foundPkg)
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

// parseStructFields parses the fields of a given go/ast.TypeSpec that represents a struct.
func (w *PackageWalker) parseStructFields(typeSpec *goast.TypeSpec, pkg *packages.Package) types.TypeInfo {
	slog.Debug("parseStructFields: 解析结构体字段", "类型名", typeSpec.Name.Name, "包名", pkg.Name, "包路径", pkg.PkgPath)
	info := types.TypeInfo{
		Name:        typeSpec.Name.Name,
		PkgName:     pkg.Name,
		ImportPath:  pkg.PkgPath,
		ImportAlias: w.findAliasForPath(pkg.PkgPath),
		Fields:      []types.StructField{},
	}

	if structType, ok := typeSpec.Type.(*goast.StructType); ok {
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

// ensureNodeExists ensures a ConversionNode exists for a given type name in the conversion graph.
func (w *PackageWalker) ensureNodeExists(typeName string) {
	if _, exists := w.graph[typeName]; !exists {
		w.graph[typeName] = &types.ConversionNode{
			Configs: make(map[string]*types.ConversionConfig),
		}
	}
}

// AddConversion adds a new ConversionConfig to the conversion graph, handling bidirectional conversions.
func (w *PackageWalker) AddConversion(cfg *types.ConversionConfig) {
	if w.graph == nil {
		return // If graph is nil, we cannot add conversions.
	}

	w.ensureNodeExists(cfg.Source.Type)
	w.ensureNodeExists(cfg.Target.Type)

	if cfg.Direction == "" {
		cfg.Direction = "both"
	}

	configKey := cfg.Source.Type + "2" + cfg.Target.Type

	if cfg.Direction == "to" || cfg.Direction == "both" {
		w.graph[cfg.Source.Type].ToConversions = AppendIfNotExists(w.graph[cfg.Source.Type].ToConversions, cfg.Target.Type)
		w.graph[cfg.Source.Type].Configs[configKey] = cfg
	}

	if cfg.Direction == "from" || cfg.Direction == "both" {
		w.graph[cfg.Target.Type].ToConversions = AppendIfNotExists(w.graph[cfg.Target.Type].ToConversions, cfg.Source.Type)

		reverseCfg := &types.ConversionConfig{
			Source: &types.EndpointConfig{
				Type:   cfg.Target.Type,
				Prefix: cfg.Target.Prefix,
				Suffix: cfg.Target.Suffix,
			},
			Target: &types.EndpointConfig{
				Type:   cfg.Source.Type,
				Prefix: cfg.Source.Prefix,
				Suffix: cfg.Source.Suffix,
			},
			Direction:           "to", // The reverse is always a simple "to"
			IgnoreFields:        cfg.IgnoreFields,
			RemapFields:         cfg.RemapFields, // Note: remap is usually one-way, might need adjustment
			TypeConversionRules: cfg.TypeConversionRules,
		}
		reverseKey := reverseCfg.Source.Type + "2" + reverseCfg.Target.Type
		w.graph[reverseCfg.Source.Type].Configs[reverseKey] = reverseCfg
	}
}
