// Package template implements the functions, types, and interfaces for the module.
package template

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"github.com/origadmin/abgen/internal/types"
)

//go:embed conversion.tpl
var conversionTemplate string

type Convert struct {
	templates *template.Template
	cacheMap  map[string]string // 加速缓存
}

func NewConverter() *Convert {
	// 使用 embed 的模板内容替代文件读取
	tpl := template.Must(template.New("conversion").Parse(conversionTemplate))

	return &Convert{
		templates: tpl,
		cacheMap:  buildCacheMap(tpl), // 预构建缓存
	}
}

// buildCacheMap 预构建模板缓存映射
func buildCacheMap(tpl *template.Template) map[string]string {
	cache := make(map[string]string)
	for _, t := range tpl.Templates() {
		if t.Name() != "" {
			cache[t.Name()] = t.Name() // 直接存储模板名称字符串
		}
	}
	return cache
}

// LoadExternalTemplates 修改后的加载方法支持文件和目录
func (c *Convert) LoadExternalTemplates(paths ...string) error {
	if len(paths) == 0 {
		return nil
	}

	// 克隆主模板
	newTpl, err := c.templates.Clone()
	if err != nil {
		return fmt.Errorf("template clone failed: %w", err)
	}

	// 处理所有路径
	for _, path := range paths {
		// 获取路径信息
		fi, err := os.Stat(path)
		if err != nil {
			continue // 忽略不存在的路径
		}

		switch {
		case fi.IsDir():
			// 处理目录（加载所有.tpl文件）
			files, err := filepath.Glob(filepath.Join(path, "*.tpl"))
			if err != nil {
				return fmt.Errorf("glob pattern error: %w", err)
			}
			for _, f := range files {
				if _, err := newTpl.ParseFiles(f); err != nil {
					return fmt.Errorf("parse %s failed: %w", f, err)
				}
			}
		default:
			// 处理单个文件
			if _, err := newTpl.ParseFiles(path); err != nil {
				return fmt.Errorf("parse %s failed: %w", path, err)
			}
		}
	}

	// 原子化更新
	c.templates = newTpl
	c.cacheMap = buildCacheMap(newTpl)
	return nil
}

// Convert 带缓存的转换方法
func (c *Convert) Convert(w io.Writer, params types.ConversionConfig) error {
	key := fmt.Sprintf("%s:%s", params.SourceType, params.TargetType)

	// 缓存查找（O(1)时间复杂度）
	templateName, exists := c.cacheMap[key]
	if !exists {
		return fmt.Errorf("unsupported conversion: %s", key)
	}
	// 直接执行目标模板1`
	return c.templates.ExecuteTemplate(w, templateName, params)
}

func (c *Convert) Render(key string, data interface{}) ([]byte, error) {
	// 先尝试使用已加载的模板
	templateName, exists := c.cacheMap[key]
	if !exists {
		return nil, fmt.Errorf("unsupported conversion: %s", key)
	}

	var buf bytes.Buffer
	if err := c.templates.ExecuteTemplate(&buf, templateName, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
