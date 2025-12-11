// Package template provides the templating engine for abgen.
package template

import (
	"bytes"
	"embed"
	"text/template"

	"github.com/origadmin/abgen/internal/model"
)

//go:embed *.tpl
var templates embed.FS

// Renderer is the interface for rendering templates.
type Renderer interface {
	Render(templateName string, data interface{}) ([]byte, error)
}

// Manager is a template manager that holds and renders templates.
type Manager struct {
	tmpl *template.Template
}

// NewManager creates a new template manager and parses the embedded templates.
func NewManager() *Manager {
	tmpl := template.Must(template.ParseFS(templates, "*.tpl"))
	return &Manager{tmpl: tmpl}
}

// Render executes the named template with the given data.
func (m *Manager) Render(templateName string, data interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := m.tmpl.ExecuteTemplate(&buf, templateName, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Data is the top-level struct passed to the template.
type Data struct {
	PackageName     string
	Types           []model.TypeInfo // Use model.TypeInfo
	TypeAliases     []string
	Funcs           []*Function
	CustomRuleFuncs []string // Add this field
}

// Function represents a single conversion function to be generated.
type Function struct {
	Name          string
	SourceType    string
	TargetType    string
	SourcePointer string
	TargetPointer string
	Conversions   []model.Field
}
