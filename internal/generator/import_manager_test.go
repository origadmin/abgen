package generator

import (
	"testing"
)

func TestImportManager_Add(t *testing.T) {
	im := NewImportManager()
	
	// Test adding first import
	alias := im.Add("github.com/example/pkg1")
	if alias != "pkg1" {
		t.Errorf("Expected alias 'pkg1', got '%s'", alias)
	}
	
	// Test adding same import again should return same alias
	alias2 := im.Add("github.com/example/pkg1")
	if alias2 != "pkg1" {
		t.Errorf("Expected alias 'pkg1', got '%s'", alias2)
	}
	
	// Test adding different import
	alias3 := im.Add("github.com/example/pkg2")
	if alias3 != "pkg2" {
		t.Errorf("Expected alias 'pkg2', got '%s'", alias3)
	}
}

func TestImportManager_ConflictHandling(t *testing.T) {
	im := NewImportManager()
	
	// Add imports that would have the same base alias
	alias1 := im.Add("github.com/example1/pkg")
	alias2 := im.Add("github.com/example2/pkg")
	
	if alias1 != "pkg" {
		t.Errorf("Expected first alias 'pkg', got '%s'", alias1)
	}
	
	if alias2 != "pkg1" {
		t.Errorf("Expected second alias 'pkg1', got '%s'", alias2)
	}
}

func TestImportManager_GetAllImports(t *testing.T) {
	im := NewImportManager()
	
	im.Add("github.com/example/pkg1")
	im.Add("github.com/example/pkg2")
	
	imports := im.GetAllImports()
	
	if len(imports) != 2 {
		t.Errorf("Expected 2 imports, got %d", len(imports))
	}
	
	// Should be sorted
	if imports[0] != "github.com/example/pkg1" {
		t.Errorf("Expected first import 'github.com/example/pkg1', got '%s'", imports[0])
	}
	
	if imports[1] != "github.com/example/pkg2" {
		t.Errorf("Expected second import 'github.com/example/pkg2', got '%s'", imports[1])
	}
}

func TestImportManager_GetAlias(t *testing.T) {
	im := NewImportManager()
	
	im.Add("github.com/example/pkg")
	
	alias := im.GetAlias("github.com/example/pkg")
	if alias != "pkg" {
		t.Errorf("Expected alias 'pkg', got '%s'", alias)
	}
	
	// Test non-existent import
	alias2 := im.GetAlias("github.com/example/nonexistent")
	if alias2 != "" {
		t.Errorf("Expected empty alias for non-existent import, got '%s'", alias2)
	}
}

func TestImportManager_String(t *testing.T) {
	im := NewImportManager()
	
	// Empty import manager should return empty string
	if im.String() != "" {
		t.Error("Expected empty string for empty import manager")
	}
	
	im.Add("github.com/example/pkg1")
	im.Add("github.com/example/pkg2")
	
	result := im.String()
	t.Logf("Result:\n%s", result)
	
	// For now, just check that it contains the right structure
	if !contains(result, "import (") || !contains(result, ")") {
		t.Error("Result should be a proper import block")
	}
}