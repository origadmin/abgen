package components

import (
	"bytes"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/origadmin/abgen/internal/model"
)

// ConcreteImportManager 实现 ImportManager 接口
type ConcreteImportManager struct {
	imports map[string]string
	aliases map[string]string
	counter int
}

// NewImportManager 创建新的导入管理器
func NewImportManager() model.ImportManager {
	return &ConcreteImportManager{
		imports: make(map[string]string),
		aliases: make(map[string]string),
		counter: 1,
	}
}

// Add 添加导入并返回要使用的别名
func (im *ConcreteImportManager) Add(importPath string) string {
	if alias, exists := im.imports[importPath]; exists {
		return alias
	}
	
	// 生成基础别名
	alias := path.Base(importPath)
	if alias == "." || alias == "" {
		alias = fmt.Sprintf("pkg%d", im.counter)
		im.counter++
	}
	
	// 处理冲突
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

// GetAlias 返回导入路径的别名
func (im *ConcreteImportManager) GetAlias(importPath string) string {
	return im.aliases[importPath]
}

// GetAllImports 将所有导入作为排序后的切片返回
func (im *ConcreteImportManager) GetAllImports() []string {
	paths := make([]string, 0, len(im.imports))
	for p := range im.imports {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// WriteImportsToBuffer 将导入块写入给定的缓冲区（辅助方法，不暴露在接口中）
func (im *ConcreteImportManager) WriteImportsToBuffer(buf *bytes.Buffer) {
	imports := im.GetAllImports()
	if len(imports) == 0 {
		return
	}
	buf.WriteString("import(\n")
	for _, importPath := range imports {
		alias := im.GetAlias(importPath)
		// 仅当别名与基础名称不同或者是生成的别名时才显示别名
		baseAlias := path.Base(importPath)
		if alias == baseAlias && !strings.HasPrefix(alias, "pkg") {
			buf.WriteString(fmt.Sprintf("\t%q\n", importPath))
		} else {
			buf.WriteString(fmt.Sprintf("\t%s %q\n", alias, importPath))
		}
	}
	buf.WriteString(")\n\n")
}

// String 返回所有导入的字符串表示
func (im *ConcreteImportManager) String() string {
	if len(im.imports) == 0 {
		return ""
	}
	
	var result string
	result += "import(\n"
	
	for _, importPath := range im.GetAllImports() {
		alias := im.imports[importPath]
		// 仅当别名与基础名称不同时才显示别名
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