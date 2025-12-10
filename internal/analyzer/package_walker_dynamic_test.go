package analyzer

import (
	"testing"
)

// TestFindTypeByFQNDynamicLoading tests the new dynamic package loading functionality.
func TestFindTypeByFQNDynamicLoading(t *testing.T) {
	walker := NewPackageWalker()

	// Test case 1: Try to find a type in standard library without preloading
	// This should trigger dynamic loading
	info, err := walker.FindTypeByFQN("time.Time")
	if err != nil {
		t.Fatalf("Expected to find time.Time, got error: %v", err)
	}
	if info.Name != "Time" || info.ImportPath != "time" {
		t.Errorf("Expected time.Time with name 'Time' and import 'time', got name='%s', import='%s'", info.Name, info.ImportPath)
	}

	// Test case 2: Try to find another type in the same package (should use cache)
	info2, err := walker.FindTypeByFQN("time.Duration")
	if err != nil {
		t.Fatalf("Expected to find time.Duration, got error: %v", err)
	}
	if info2.Name != "Duration" || info2.ImportPath != "time" {
		t.Errorf("Expected time.Duration with name 'Duration' and import 'time', got name='%s', import='%s'", info2.Name, info2.ImportPath)
	}

	// Test case 3: Verify that failed packages are tracked
	// Try to load a non-existent package
	_, err = walker.FindTypeByFQN("non.existent.package.12345.Type")
	if err == nil {
		t.Error("Expected error when trying to load non-existent package")
	}

	// Try the same non-existent package again - should fail fast due to tracking
	_, err = walker.FindTypeByFQN("non.existent.package.12345.AnotherType")
	if err == nil {
		t.Error("Expected error when trying to load previously failed package")
	}

	if !walker.failedLoads["non.existent.package.12345"] {
		t.Error("Expected non.existent.package.12345 to be in failedLoads")
	}
}

// TestFindTypeInLoadedPkgs tests the findTypeInLoadedPkgs method
func TestFindTypeInLoadedPkgs(t *testing.T) {
	walker := NewPackageWalker()

	// Test with invalid FQN (no dot)
	_, err := walker.findTypeInLoadedPkgs("invalid_fqn_without_dot")
	if err == nil {
		t.Error("Expected error for invalid FQN")
	}

	// Test with empty package path
	_, err = walker.findTypeInLoadedPkgs(".Type")
	if err == nil {
		t.Error("Expected error for empty package path")
	}

	// Test with empty type name
	_, err = walker.findTypeInLoadedPkgs("package.")
	if err == nil {
		t.Error("Expected error for empty type name")
	}

	// Test with non-existent type in non-loaded package
	_, err = walker.findTypeInLoadedPkgs("non.loaded.package.Type")
	if err == nil {
		t.Error("Expected error for non-loaded package")
	}
}

// TestLoadMissingPackage tests the loadMissingPackage method
func TestLoadMissingPackage(t *testing.T) {
	walker := NewPackageWalker()

	// Test loading a standard library package
	err := walker.loadMissingPackage("time")
	if err != nil {
		t.Fatalf("Failed to load 'time' package: %v", err)
	}

	// Verify the package is now loaded
	found := false
	for _, pkg := range walker.pkgs {
		if pkg.PkgPath == "time" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'time' package to be loaded")
	}

	// Test loading the same package again (should not error)
	err = walker.loadMissingPackage("time")
	if err != nil {
		t.Errorf("Unexpected error when loading already loaded 'time' package: %v", err)
	}

	// Test loading a non-existent package
	err = walker.loadMissingPackage("non.existent.package.12345")
	if err == nil {
		t.Error("Expected error when loading non-existent package")
	}
}

// TestFindTypeByFQNTypeResolving tests that type resolving works correctly after dynamic loading
func TestFindTypeByFQNTypeResolving(t *testing.T) {
	walker := NewPackageWalker()

	// Test resolving a struct type with fields
	info, err := walker.FindTypeByFQN("time.Time")
	if err != nil {
		t.Fatalf("Expected to find time.Time, got error: %v", err)
	}

	if info.Kind != Struct {
		t.Errorf("Expected time.Time to be a struct, got kind %v", info.Kind)
	}

	// time.Time should have some fields (like wall, ext)
	if len(info.Fields) == 0 {
		t.Error("Expected time.Time to have fields")
	}

	// Test resolving a pointer type
	info2, err := walker.FindTypeByFQN("encoding/json.RawMessage")
	if err != nil {
		t.Fatalf("Expected to find encoding/json.RawMessage, got error: %v", err)
	}

	// RawMessage is defined as type RawMessage []byte, so it should be a slice
	if info2.Kind != Slice {
		t.Errorf("Expected encoding/json.RawMessage to be a slice, got kind %v", info2.Kind)
	}
}

// TestFindTypeByFQNAliases tests that type aliases are handled correctly
func TestFindTypeByFQNAliases(t *testing.T) {
	walker := NewPackageWalker()

	// Test finding a type alias (byte is an alias for uint8)
	info, err := walker.FindTypeByFQN("byte")
	if err != nil {
		t.Fatalf("Expected to find byte type, got error: %v", err)
	}

	// byte should be a basic type (primitive)
	if info.Kind != Primitive {
		t.Errorf("Expected byte to be a primitive type, got kind %v", info.Kind)
	}

	// Test finding a more complex alias
	info2, err := walker.FindTypeByFQN("error")
	if err != nil {
		t.Fatalf("Expected to find error type, got error: %v", err)
	}

	// error should be an interface type
	if info2.Kind != Interface {
		t.Errorf("Expected error to be an interface type, got kind %v", info2.Kind)
	}
}