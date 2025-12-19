package components

import (
	"bytes"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/origadmin/abgen/internal/model"
)

// ImportManager implements the ImportManager interface.
type ImportManager struct {
	imports map[string]string
	aliases map[string]string
	counter int
}

// NewImportManager creates a new import manager.
func NewImportManager() model.ImportManager {
	return &ImportManager{
		imports: make(map[string]string),
		aliases: make(map[string]string),
		counter: 1,
	}
}

// Add adds an import and returns the alias to be used.
func (im *ImportManager) Add(importPath string) string {
	if alias, exists := im.imports[importPath]; exists {
		return alias
	}

	// Generate a base alias.
	alias := path.Base(importPath)
	if alias == "." || alias == "" {
		alias = fmt.Sprintf("pkg%d", im.counter)
		im.counter++
	}

	// Handle conflicts.
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

// AddAs adds an import with a specific alias.
func (im *ImportManager) AddAs(importPath, alias string) string {
	im.imports[importPath] = alias
	im.aliases[importPath] = alias
	return alias
}

// GetAlias returns the alias for an import path.
func (im *ImportManager) GetAlias(importPath string) (string, bool) {
	alias, ok := im.aliases[importPath]
	return alias, ok
}

// GetAllImports returns all imports as a map of path to alias.
func (im *ImportManager) GetAllImports() map[string]string {
	return im.imports
}

// WriteImportsToBuffer writes the import block to the given buffer (helper method, not exposed in the interface).
func (im *ImportManager) WriteImportsToBuffer(buf *bytes.Buffer) {
	imports := im.GetAllImports()
	if len(imports) == 0 {
		return
	}

	// Sort paths for consistent output
	paths := make([]string, 0, len(imports))
	for p := range imports {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	buf.WriteString("import(\n")
	for _, importPath := range paths {
		alias := im.imports[importPath]
		// Only show the alias if it's different from the base name or is a generated alias.
		baseAlias := path.Base(importPath)
		if alias == baseAlias && !strings.HasPrefix(alias, "pkg") {
			buf.WriteString(fmt.Sprintf("\t%q\n", importPath))
		} else {
			buf.WriteString(fmt.Sprintf("\t%s %q\n", alias, importPath))
		}
	}
	buf.WriteString(")\n\n")
}

// String returns the string representation of all imports.
func (im *ImportManager) String() string {
	if len(im.imports) == 0 {
		return ""
	}

	var result string
	result += "import(\n"

	// Sort paths for consistent output
	paths := make([]string, 0, len(im.imports))
	for p := range im.imports {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, importPath := range paths {
		alias := im.imports[importPath]
		// Only show the alias if it's different from the base name.
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
