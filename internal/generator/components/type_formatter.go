package components

import (
	"fmt"
	"go/types"

	"github.com/origadmin/abgen/internal/model"
)

// TypeFormatter is responsible for converting a types.Type into its string representation.
type TypeFormatter struct {
	aliasManager  model.AliasManager
	importManager model.ImportManager
	typeInfos     map[string]*model.TypeInfo
}

// NewTypeFormatter creates a new TypeFormatter.
func NewTypeFormatter(
	analysisResult *model.AnalysisResult,
	aliasManager model.AliasManager,
	importManager model.ImportManager,
) model.TypeFormatter {
	return &TypeFormatter{
		aliasManager:  aliasManager,
		importManager: importManager,
		typeInfos:     analysisResult.TypeInfos,
	}
}

// Format converts the given Go type into its string representation.
func (f *TypeFormatter) Format(info *model.TypeInfo) string {
	return f.formatTypeInfo(info, true) // Check for aliases
}

// FormatWithoutAlias converts the given Go type into its string representation without using aliases.
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
		if info.Name != "" {
			return f.qualifiedNameFromInfo(info)
		}
		return "struct{}"
	default:
		if info.Original != nil {
			return types.TypeString(info.Original.Type(), f.qualifier)
		}
		return info.TypeString()
	}
}

// qualifiedNameFromInfo ensures that when a type's qualified name is generated,
// its package is added to the import manager.
func (f *TypeFormatter) qualifiedNameFromInfo(info *model.TypeInfo) string {
	if info.ImportPath == "" {
		return info.Name // Built-in type or local
	}

	// Add the import path to the manager and get the alias to use.
	// The manager handles conflicts and ensures the path is only added once.
	pkgAlias := f.importManager.Add(info.ImportPath)

	return fmt.Sprintf("%s.%s", pkgAlias, info.Name)
}

func (f *TypeFormatter) qualifier(pkg *types.Package) string {
	// This is a fallback for types.TypeString. It should also ensure the package is imported.
	return f.importManager.Add(pkg.Path())
}
