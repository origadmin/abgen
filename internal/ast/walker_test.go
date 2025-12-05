package ast

import (
	"log/slog"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/origadmin/abgen/internal/types"
	"golang.org/x/tools/go/packages"
)

func loadTestPackages(t *testing.T) []*packages.Package {
	t.Helper()
	testDir, err := filepath.Abs("./testdata")
	if err != nil {
		t.Fatalf("Failed to get absolute path for testdata: %v", err)
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports,
		Dir:  testDir,
	}
	pkgs, err := packages.Load(cfg,
		"github.com/origadmin/abgen/internal/ast/testdata",
		"github.com/origadmin/abgen/internal/testdata/ent",
		"github.com/origadmin/abgen/internal/testdata/typespb",
	)
	if err != nil {
		t.Fatalf("Failed to load packages: %v", err)
	}

	if packages.PrintErrors(pkgs) > 0 {
		t.Fatal("Errors occurred while loading packages.")
	}
	return pkgs
}

func findPackage(pkgs []*packages.Package, suffix string) *packages.Package {
	for _, pkg := range pkgs {
		if strings.HasSuffix(pkg.PkgPath, suffix) {
			return pkg
		}
	}
	return nil
}

func TestWalker_Parser(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	allPkgs := loadTestPackages(t)
	targetPkg := findPackage(allPkgs, "testdata")
	if targetPkg == nil {
		t.Fatal("Could not find the target testdata package.")
	}

	graph := make(types.ConversionGraph)
	walker := NewPackageWalker(graph)
	walker.AddPackages(allPkgs...)

	err := walker.WalkPackage(targetPkg)
	if err != nil {
		t.Fatalf("Walker.WalkPackage() failed: %v", err)
	}
	
	t.Run("PackagePairDirective", func(t *testing.T) {
		if len(walker.PackageConfigs) != 1 {
			t.Fatalf("Expected 1 PackageConversionConfig, got %d", len(walker.PackageConfigs))
		}
		pkgCfg := walker.PackageConfigs[0]

		if pkgCfg.SourcePackage != "github.com/origadmin/abgen/internal/testdata/ent" {
			t.Errorf("Expected SourcePackage %q, got %q", "github.com/origadmin/abgen/internal/testdata/ent", pkgCfg.SourcePackage)
		}
		if pkgCfg.TargetPackage != "github.com/origadmin/abgen/internal/testdata/typespb" {
			t.Errorf("Expected TargetPackage %q, got %q", "github.com/origadmin/abgen/internal/testdata/typespb", pkgCfg.TargetPackage)
		}
		if pkgCfg.SourceSuffix != "Ent" {
			t.Errorf("Expected SourceSuffix 'Ent', got %q", pkgCfg.SourceSuffix)
		}
		if pkgCfg.TargetSuffix != "PB" {
			t.Errorf("Expected TargetSuffix 'PB', got %q", pkgCfg.TargetSuffix)
		}
		if pkgCfg.Direction != "both" {
			t.Errorf("Expected Direction 'both', got %q", pkgCfg.Direction)
		}

		expectedIgnores := map[string]bool{"Password": true, "Salt": true}
		if !reflect.DeepEqual(pkgCfg.IgnoreFields, expectedIgnores) {
			t.Errorf("Expected ignored fields %v, got %v", expectedIgnores, pkgCfg.IgnoreFields)
		}
		
		expectedRemaps := map[string]string{"Roles": "Edges.Roles", "RoleIDs": "Edges.Roles.ID"}
		if !reflect.DeepEqual(pkgCfg.RemapFields, expectedRemaps) {
			t.Errorf("Expected remap fields %v, got %v", expectedRemaps, pkgCfg.RemapFields)
		}

		expectedRules := []types.TypeConversionRule{
			{SourceTypeName: "github.com/origadmin/abgen/internal/testdata/ent.Status", TargetTypeName: "builtin.string", ConvertFunc: "ConvertStatusToString"},
			{SourceTypeName: "builtin.string", TargetTypeName: "github.com/origadmin/abgen/internal/testdata/ent.Status", ConvertFunc: "ConvertString2Status"},
		}
		if !reflect.DeepEqual(pkgCfg.TypeConversionRules, expectedRules) {
			t.Errorf("Expected rules %v, got %v", expectedRules, pkgCfg.TypeConversionRules)
		}
	})

	t.Run("ConvertDirectives", func(t *testing.T) {
		if len(graph) != 2 {
			t.Fatalf("Expected 2 nodes in graph, got %d", len(graph))
		}

		key1 := "github.com/origadmin/abgen/internal/testdata/ent.User2github.com/origadmin/abgen/internal/testdata/typespb.User"
		node1, ok := graph["github.com/origadmin/abgen/internal/testdata/ent.User"]
		if !ok {
			t.Fatalf("Graph missing node for ent.User")
		}
		cfg1, ok := node1.Configs[key1]
		if !ok {
			t.Fatalf("Node for ent.User missing config for key: %s", key1)
		}

		// Assert EndpointConfig usage
		if cfg1.Source.Type != "github.com/origadmin/abgen/internal/testdata/ent.User" {
			t.Errorf("Cfg1: Expected source type %q, got %q", "github.com/origadmin/abgen/internal/testdata/ent.User", cfg1.Source.Type)
		}
		if cfg1.Target.Type != "github.com/origadmin/abgen/internal/testdata/typespb.User" {
			t.Errorf("Cfg1: Expected target type %q, got %q", "github.com/origadmin/abgen/internal/testdata/typespb.User", cfg1.Target.Type)
		}

		expectedIgnores1 := map[string]bool{"CreatedAt": true, "UpdatedAt": true}
		if !reflect.DeepEqual(cfg1.IgnoreFields, expectedIgnores1) {
			t.Errorf("Cfg1: Expected ignored fields %v, got %v", expectedIgnores1, cfg1.IgnoreFields)
		}
		if cfg1.Direction != "both" {
			t.Errorf("Cfg1: Expected direction 'both', got %q", cfg1.Direction)
		}

		// Remap fields are not present in this config based on testdata.
		if len(cfg1.RemapFields) != 0 {
			t.Errorf("Cfg1: Expected 0 remap fields, got %d", len(cfg1.RemapFields))
		}

		revKey1 := "github.com/origadmin/abgen/internal/testdata/typespb.User2github.com/origadmin/abgen/internal/testdata/ent.User"
		revNode1, ok := graph["github.com/origadmin/abgen/internal/testdata/typespb.User"]
		if !ok {
			t.Fatalf("Graph missing reverse node for typespb.User")
		}
		revCfg1, ok := revNode1.Configs[revKey1]
		if !ok {
			t.Fatalf("Reverse node for typespb.User missing config for key: %s", revKey1)
		}
		if revCfg1.Direction != "to" {
			t.Errorf("revCfg1: Expected direction 'to', got %q", revCfg1.Direction)
		}
		// Check that the reverse config correctly inherited remap fields from original
		if !reflect.DeepEqual(revCfg1.RemapFields, cfg1.RemapFields) {
			t.Errorf("Reverse config remap fields mismatch: expected %v, got %v", cfg1.RemapFields, revCfg1.RemapFields)
		}
		// Check that the reverse config has correct prefixes/suffixes
		if revCfg1.Source.Suffix != "" || revCfg1.Target.Suffix != "" {
			t.Errorf("Reverse config: Expected empty suffixes, got Source.Suffix=%q, Target.Suffix=%q", revCfg1.Source.Suffix, revCfg1.Target.Suffix)
		}
	})

	t.Run("ConvertDirectives_Directional", func(t *testing.T) {
		// This is for SingleDirection -> SingleDirectionPB (direction=from)
		// The logic for "from" creates a reverse conversion config.
		key2 := "github.com/origadmin/abgen/internal/testdata/typespb.User2github.com/origadmin/abgen/internal/testdata/ent.User"
		node2, ok := graph["github.com/origadmin/abgen/internal/testdata/typespb.User"]
		if !ok {
			t.Fatalf("Graph missing node for typespb.User for the 'from' directive")
		}
		
		// The original forward config (SingleDirection -> SingleDirectionPB) should NOT exist in the graph.
		// It only exists as a basis for the reverse config.
		// So we check the one created by AddConversion's internal reversal.
		cfg2, ok := node2.Configs[key2]
		if !ok {
			t.Fatalf("Node for typespb.User missing config for key: %s", key2)
		}

		if cfg2.Direction != "to" {
			t.Errorf("Cfg2: Expected direction of reversed config to be 'to', got %q", cfg2.Direction)
		}
		if cfg2.Source.Type != "github.com/origadmin/abgen/internal/testdata/typespb.User" {
			t.Errorf("Cfg2: Expected source type to be typespb.User, got %q", cfg2.Source.Type)
		}
		if cfg2.Target.Type != "github.com/origadmin/abgen/internal/testdata/ent.User" {
			t.Errorf("Cfg2: Expected target type to be ent.User, got %q", cfg2.Target.Type)
		}
	})
}