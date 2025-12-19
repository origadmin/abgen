package components

import (
	"fmt"
	"go/types"

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
func (f *TypeFormatter) Format(info *model.TypeInfo) string {
	return f.formatTypeInfo(info, true) // Check for aliases
}

// FormatWithoutAlias converts the given Go type into its string representation without using aliases.
// This is used when we need the original type name for alias generation.
func (f *TypeFormatter) FormatWithoutAlias(info *model.TypeInfo) string {
	return f.formatTypeInfo(info, false) // Do not check for aliases
}

func (f *TypeFormatter) formatTypeInfo(info *model.TypeInfo, checkAlias bool) string {
	if info == nil {
		return "nil"
	}

	if checkAlias {
		if alias, ok := f.aliasManager.GetAlias(info); ok {
			return alias
		}
	}

	switch info.Kind {
	case model.Primitive:
		return info.Name
	case model.Pointer:
		return "*" + f.formatTypeInfo(info.Underlying, checkAlias)
	case model.Named:
		return f.qualifiedNameFromInfo(info)
	case model.Slice:
		return "[]" + f.formatTypeInfo(info.Underlying, checkAlias)
	case model.Array:
		return fmt.Sprintf("[%d]%s", info.ArrayLen, f.formatTypeInfo(info.Underlying, checkAlias))
	case model.Map:
		key := f.formatTypeInfo(info.KeyType, checkAlias)
		elem := f.formatTypeInfo(info.Underlying, checkAlias)
		return fmt.Sprintf("map[%s]%s", key, elem)
	case model.Interface:
		if info.Name == "interface{}" || info.Name == "any" {
			return "any"
		}
		if info.Name == "error" {
			return "error"
		}
		return info.TypeString() // Fallback
	case model.Struct:
		// This case is for anonymous structs, which are rare in this context.
		// Named structs are handled by model.Named.
		return "struct{}"
	default:
		// Fallback for types not explicitly handled (e.g., Chan, Func, etc.)
		if info.Original != nil {
			return types.TypeString(info.Original.Type(), f.qualifier)
		}
		return info.TypeString()
	}
}

func (f *TypeFormatter) qualifiedNameFromInfo(info *model.TypeInfo) string {
	if info.ImportPath == "" {
		return info.Name // Built-in type
	}

	// Find the package from the import manager to get the correct alias
	pkgAlias, found := f.importManager.GetAlias(info.ImportPath)
	if !found {
		// This should not happen if all types are analyzed correctly
		return info.Name
	}

	return fmt.Sprintf("%s.%s", pkgAlias, info.Name)
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
