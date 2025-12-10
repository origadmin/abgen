// Package generator provides import management for generated code.
package generator

import (
	"fmt"
	"path"
	"sort"
)

// ImportManager manages imports for generated code.
type ImportManager struct {
	imports map[string]string
	aliases map[string]string
	counter int
}

// NewImportManager creates a new ImportManager.
func NewImportManager() *ImportManager {
	return &ImportManager{
		imports: make(map[string]string),
		aliases: make(map[string]string),
		counter: 1,
	}
}

// Add adds an import and returns the alias to use.
func (im *ImportManager) Add(importPath string) string {
	if alias, exists := im.imports[importPath]; exists {
		return alias
	}
	
	// Generate base alias
	alias := path.Base(importPath)
	if alias == "." || alias == "" {
		alias = fmt.Sprintf("pkg%d", im.counter)
		im.counter++
	}
	
	// Handle conflicts
	originalAlias := alias
	conflictCounter := 1
	for {
		conflict := false
		for _, existingAlias := range im.imports {
			if existingAlias == alias {
				conflict = true
				break
			}
		}
		if !conflict {
			break
		}
		alias = fmt.Sprintf("%s%d", originalAlias, conflictCounter)
		conflictCounter++
	}
	
	im.imports[importPath] = alias
	im.aliases[importPath] = alias
	return alias
}

// GetAlias returns the alias for an import path.
func (im *ImportManager) GetAlias(importPath string) string {
	return im.aliases[importPath]
}

// GetAllImports returns all imports as a sorted slice.
func (im *ImportManager) GetAllImports() []string {
	paths := make([]string, 0, len(im.imports))
	for p := range im.imports {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// WriteImportBlock writes the import block to the given buffer.
func (im *ImportManager) WriteImportBlock(buf interface{}) {
	// This would need to be adapted to work with bytes.Buffer or similar
	// For now, this is a placeholder implementation
}

// String returns a string representation of all imports.
func (im *ImportManager) String() string {
	if len(im.imports) == 0 {
		return ""
	}
	
	var result string
	result += "import (\n"
	
	for _, importPath := range im.GetAllImports() {
		alias := im.imports[importPath]
		// Only show alias if it's different from the base name
		baseAlias := path.Base(importPath)
		if alias == baseAlias {
			result += fmt.Sprintf("\t%q\n", importPath)
		} else {
			result += fmt.Sprintf("\t%s %q\n", alias, importPath)
		}
	}
	
	result += ")\n"
	return result
}