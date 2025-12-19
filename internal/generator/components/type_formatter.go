package components

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/origadmin/abgen/internal/model"
)

// TypeFormatter is responsible for converting a types.Type into its string representation.
// It consults an AliasManager to determine if an alias should be used for a type
// and uses an ImportManager to handle package qualification.
type TypeFormatter struct {
	aliasManager  model.AliasManager
	importManager model.ImportManager
	typeInfos     map[string]*model.TypeInfo // Added to resolve TypeInfo from types.Type
}

// NewTypeFormatter creates a new TypeFormatter.
func NewTypeFormatter(aliasManager model.AliasManager, importManager model.ImportManager, typeInfos map[string]*model.TypeInfo) *TypeFormatter {
	return &TypeFormatter{
		aliasManager:  aliasManager,
		importManager: importManager,
		typeInfos:     typeInfos,
	}
}

// Format converts the given Go type into its string representation.
// It's a sophisticated version of types.TypeString that is aware of the generator's aliasing and import context.
func (f *TypeFormatter) Format(typ types.Type) string {
	return f.formatType(typ, true) // Check for aliases
}

// FormatWithoutAlias converts the given Go type into its string representation without using aliases.
// This is used when we need the original type name for alias generation.
func (f *TypeFormatter) FormatWithoutAlias(typ types.Type) string {
	return f.formatType(typ, false) // Do not check for aliases
}

func (f *TypeFormatter) formatType(typ types.Type, checkAlias bool) string {
	// Try to get model.TypeInfo for the current Go type
	typeInfo := f.getTypeInfoFromGoType(typ)

	if checkAlias && typeInfo != nil {
		if alias, ok := f.aliasManager.GetAlias(typeInfo); ok {
			return alias
		}
	}

	switch t := typ.(type) {
	case *types.Basic:
		return t.String()
	case *types.Pointer:
		return "*" + f.formatType(t.Elem(), checkAlias)
	case *types.Named:
		// If alias check failed or not requested, format as qualified name
		return f.qualifiedName(t)
	case *types.Slice:
		return "[]" + f.formatType(t.Elem(), checkAlias)
	case *types.Array:
		return fmt.Sprintf("[%d]%s", t.Len(), f.formatType(t.Elem(), checkAlias))
	case *types.Map:
		key := f.formatType(t.Key(), checkAlias)
		elem := f.formatType(t.Elem(), checkAlias)
		return fmt.Sprintf("map[%s]%s", key, elem)
	case *types.Interface:
		// Handle `error` and `any` as special cases
		if t.NumEmbeddeds() == 0 && t.NumExplicitMethods() == 0 {
			return "any" // for empty interface
		}
		// A bit of a simplification, but good for most cases like `error`
		if named, ok := typ.(*types.Named); ok && named.Obj().Pkg() == nil && named.Obj().Name() == "error" {
			return "error"
		}
		return t.String() // fallback for other interfaces
	case *types.Signature:
		params := make([]string, t.Params().Len())
		for i := 0; i < t.Params().Len(); i++ {
			params[i] = f.formatType(t.Params().At(i).Type(), checkAlias)
		}
		results := make([]string, t.Results().Len())
		for i := 0; i < t.Results().Len(); i++ {
			results[i] = f.formatType(t.Results().At(i).Type(), checkAlias)
		}
		paramStr := strings.Join(params, ", ")
		resultStr := strings.Join(results, ", ")
		if len(results) > 1 {
			resultStr = "(" + resultStr + ")"
		}
		return fmt.Sprintf("func(%s) %s", paramStr, resultStr)

	default:
		// Fallback for types not explicitly handled (e.g., Chan, Struct, etc.)
		return types.TypeString(typ, f.qualifier)
	}
}

func (f *TypeFormatter) qualifiedName(named *types.Named) string {
	pkg := named.Obj().Pkg()
	if pkg == nil {
		return named.Obj().Name() // Built-in type
	}

	// Use the import manager to get the correct package name (it might be aliased)
	pkgName := f.importManager.PackageName(pkg)
	if pkgName == "" {
		// Should not happen if imports are managed correctly
		return named.Obj().Name()
	}

	return fmt.Sprintf("%s.%s", pkgName, named.Obj().Name())
}

// qualifier is a helper for the fallback types.TypeString function.
func (f *TypeFormatter) qualifier(pkg *types.Package) string {
	return f.importManager.PackageName(pkg)
}

// getTypeInfoFromGoType attempts to find a model.TypeInfo for a given Go types.Type.
// This is crucial for alias lookup for composite types.
func (f *TypeFormatter) getTypeInfoFromGoType(goType types.Type) *model.TypeInfo {
	// For named types, we can directly construct the unique key
	if named, ok := goType.(*types.Named); ok {
		if named.Obj().Pkg() != nil {
			uniqueKey := named.Obj().Pkg().Path() + "." + named.Obj().Name()
			if info, exists := f.typeInfos[uniqueKey]; exists {
				return info
			}
		}
	}

	// For composite types, we need to construct a unique key that represents their structure.
	// This is a simplified approach and might need to be more robust for complex scenarios.
	// The TypeInfo for composite types should ideally be pre-populated during analysis.
	// For now, we'll try to match based on string representation if not a named type.
	// This part might need further refinement in the TypeAnalyzer.
	uniqueKey := model.GenerateUniqueKeyFromGoType(goType)
	if info, exists := f.typeInfos[uniqueKey]; exists {
		return info
	}

	return nil
}
