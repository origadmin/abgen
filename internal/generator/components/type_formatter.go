package components

import (
	"fmt"
	"go/types"
	"strings"
)

// TypeFormatter is responsible for converting a types.Type into its string representation.
// It consults an AliasManager to determine if an alias should be used for a named type
// and uses an ImportManager to handle package qualification.
type TypeFormatter struct {
	aliasManager  *AliasManager
	importManager *ImportManager
}

// NewTypeFormatter creates a new TypeFormatter.
func NewTypeFormatter(aliasManager *AliasManager, importManager *ImportManager) *TypeFormatter {
	return &TypeFormatter{
		aliasManager:  aliasManager,
		importManager: importManager,
	}
}

// Format converts the given Go type into its string representation.
// It's a sophisticated version of types.TypeString that is aware of the generator's aliasing and import context.
func (f *TypeFormatter) Format(typ types.Type) string {
	return f.formatType(typ)
}

func (f *TypeFormatter) formatType(typ types.Type) string {
	switch t := typ.(type) {
	case *types.Basic:
		return t.String()
	case *types.Pointer:
		return "*" + f.formatType(t.Elem())
	case *types.Named:
		// This is the core logic: check for an alias first.
		if alias, ok := f.aliasManager.GetAlias(t); ok {
			return alias
		}
		// If no alias, use the qualified name.
		return f.qualifiedName(t)
	case *types.Slice:
		return "[]" + f.formatType(t.Elem())
	case *types.Array:
		return fmt.Sprintf("[%d]%s", t.Len(), f.formatType(t.Elem()))
	case *types.Map:
		key := f.formatType(t.Key())
		elem := f.formatType(t.Elem())
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
			params[i] = f.formatType(t.Params().At(i).Type())
		}
		results := make([]string, t.Results().Len())
		for i := 0; i < t.Results().Len(); i++ {
			results[i] = f.formatType(t.Results().At(i).Type())
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
