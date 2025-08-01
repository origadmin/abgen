package template

import (
	"bytes"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed generator.tpl
var generatorTemplate string // generator.tpl

const generatorTemplateName = "generator"

type Renderer interface {
	Render(templateName string, data interface{}) ([]byte, error)
}

type Generator struct {
	templates map[string]*template.Template
}

// Load 加载模板
func (tm *Generator) Load(name string) error {
	var tmplContent string

	switch name {
	case generatorTemplateName:
		tmplContent = generatorTemplate
	default:
		return fmt.Errorf("未知模板: %s", name)
	}

	// 解析模板
	tmpl, err := template.New(name).Parse(tmplContent)
	if err != nil {
		return err
	}

	tm.templates[name] = tmpl
	return nil
}

// Render 渲染模板
func (tm *Generator) Render(name string, data interface{}) ([]byte, error) {
	if name == "" {
		name = generatorTemplateName
	}
	// 先尝试使用已加载的模板
	tmpl, exists := tm.templates[name]
	if !exists {
		// 如果没有加载，尝试加载内置模板
		if err := tm.Load(generatorTemplateName); err != nil {
			return nil, err
		}
		tmpl = tm.templates[name]
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func NewManager() Renderer {
	return &Generator{
		templates: make(map[string]*template.Template),
	}
}

// LoadFromDir 从目录加载自定义转换模板
func (tm *Generator) LoadFromDir(dir string) error {
	if dir == "" {
		return nil
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("读取模板目录失败: %w", err)
	}

	// 创建主转换模板
	customTmpl := template.New("custom_conversion")

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".tpl") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			slog.Info("读取模板文件失败", "文件", file.Name(), "错误", err)
			continue
		}

		_, err = customTmpl.Parse(string(content))
		if err != nil {
			slog.Info("模板内容", "内容", string(content))
		}
	}

	// 存储自定义模板
	tm.templates["custom_conversion"] = customTmpl
	return nil
}
