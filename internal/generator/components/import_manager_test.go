package components

import (
	"reflect"
	"testing"
)

func TestImportManager_Add(t *testing.T) {
	im := NewImportManager()

	// Test adding a new package
	alias := im.Add("fmt")
	if alias != "fmt" {
		t.Errorf("Expected alias 'fmt', got '%s'", alias)
	}

	// Test adding it again, should return the same alias
	alias2 := im.Add("fmt")
	if alias2 != "fmt" {
		t.Errorf("Expected second add to return alias 'fmt', got '%s'", alias2)
	}

	// Test adding a package with a path
	alias3 := im.Add("path/filepath")
	if alias3 != "filepath" {
		t.Errorf("Expected alias 'filepath', got '%s'", alias3)
	}
}

func TestImportManager_AddAs(t *testing.T) {
	im := NewImportManager()
	im.AddAs("github.com/google/uuid", "uuid")

	alias, ok := im.GetAlias("github.com/google/uuid")
	if !ok || alias != "uuid" {
		t.Errorf("Expected to get alias 'uuid', got '%s' (found: %v)", alias, ok)
	}
}

func TestImportManager_ConflictResolution(t *testing.T) {
	im := NewImportManager()

	alias1 := im.Add("a/b/c") // should be 'c'
	if alias1 != "c" {
		t.Errorf("Expected first alias to be 'c', got '%s'", alias1)
	}

	alias2 := im.Add("d/e/c") // should be 'c1' due to conflict
	if alias2 != "c1" {
		t.Errorf("Expected conflicting alias to be 'c1', got '%s'", alias2)
	}

	alias3 := im.Add("f/g/c") // should be 'c2'
	if alias3 != "c2" {
		t.Errorf("Expected second conflicting alias to be 'c2', got '%s'", alias3)
	}
}

func TestImportManager_GetAllImports(t *testing.T) {
	im := NewImportManager()
	im.Add("fmt")
	im.Add("path/filepath")
	im.AddAs("github.com/google/uuid", "uuid")

	all := im.GetAllImports()
	expected := map[string]string{
		"fmt":                 "fmt",
		"path/filepath":       "filepath",
		"github.com/google/uuid": "uuid",
	}

	if !reflect.DeepEqual(all, expected) {
		t.Errorf("GetAllImports mismatch:\ngot:  %v\nwant: %v", all, expected)
	}
}
