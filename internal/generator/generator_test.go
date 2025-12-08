package generator

import (
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerator_EndToEnd(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	g := NewGenerator()

	outputDir := "testoutput"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(outputDir) })

	g.Output = outputDir

	sourceDir, err := filepath.Abs("../../testdata/directives")
	if err != nil {
		t.Fatalf("Failed to get absolute path for source dir: %v", err)
	}

	if err := g.ParseSource(sourceDir); err != nil {
		t.Fatalf("ParseSource() failed: %v", err)
	}

	if err := g.Generate(); err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	generatedFile := filepath.Join(outputDir, "directives.gen.go")
	content, err := os.ReadFile(generatedFile)
	if err != nil {
		t.Fatalf("Failed to read generated file: %v", err)
	}

	generatedCode := string(content)
	t.Logf("Generated Code:\n%s", generatedCode)

	t.Run("GeneratedCodeValidation", func(t *testing.T) {
		// Check package and imports
		if !strings.Contains(generatedCode, "package directives") {
			t.Error("expected package 'directives'")
		}
		if !strings.Contains(generatedCode, `ent "github.com/origadmin/abgen/testdata/exps/ent"`) {
			t.Error("expected import for 'ent' package")
		}
		if !strings.Contains(generatedCode, `typespb "github.com/origadmin/abgen/testdata/exps/typespb"`) {
			t.Error("expected import for 'typespb' package")
		}

		// Check type aliases from package-pair
		if !strings.Contains(generatedCode, "type UserEnt = ent.User") {
			t.Error("expected 'UserEnt' type alias")
		}
		if !strings.Contains(generatedCode, "type UserPB = typespb.User") {
			t.Error("expected 'UserPB' type alias")
		}

		// Check function signatures using aliases
		expectedFunc1 := "func ConvertUserEntToUserPB(src *UserEnt) *UserPB"
		if !strings.Contains(generatedCode, expectedFunc1) {
			t.Errorf("missing expected function: %s", expectedFunc1)
		}

		// Check file-level ignore rules
		if strings.Contains(generatedCode, "dst.Password =") {
			t.Error("should not have conversion for 'Password' (ignored at file-level)")
		}
		if strings.Contains(generatedCode, "dst.Salt =") {
			t.Error("should not have conversion for 'Salt' (ignored at file-level)")
		}

		// Check type-level ignore rules
		if strings.Contains(generatedCode, "dst.CreatedAt =") {
			t.Error("should not have conversion for 'CreatedAt' (ignored at type-level)")
		}

		// Check file-level custom rule
		if !strings.Contains(generatedCode, "dst.Status = ConvertStatusToString(src.Status)") {
			t.Error("missing custom rule conversion for 'Status'")
		}

		// Check remap rule
		if !strings.Contains(generatedCode, "dst.Roles = src.Edges.Roles") {
			t.Error("missing remap conversion for 'Roles'")
		}
		if !strings.Contains(generatedCode, "dst.RoleIDs = src.Edges.Roles.ID") {
			t.Error("missing remap conversion for 'RoleIDs'")
		}
	})
}

func TestGenerator_SystemDTOBug(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	g := NewGenerator()

	outputDir := "testoutput"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(outputDir) })

	g.Output = outputDir

	sourceDir, err := filepath.Abs("../../testdata/core/system_dto_bug")
	if err != nil {
		t.Fatalf("Failed to get absolute path for source dir: %v", err)
	}

	if err := g.ParseSource(sourceDir); err != nil {
		t.Fatalf("ParseSource() failed: %v", err)
	}

	if err := g.Generate(); err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	generatedFile := filepath.Join(outputDir, "system_dto_bug.gen.go") // Expecting this filename based on package name
	content, err := os.ReadFile(generatedFile)
	if err != nil {
		t.Fatalf("Failed to read generated file: %v", err)
	}

	generatedCode := string(content)
	t.Logf("Generated Code for SystemDTOBug:\n%s", generatedCode)

	t.Run("InvalidTypePBGeneration", func(t *testing.T) {
		if strings.Contains(generatedCode, "type PB = ") {
			t.Errorf("Generated code contains invalid 'type PB = ' alias. Content:\n%s", generatedCode)
		}
	})

	t.Run("UndefinedResourceEdgesSkipped", func(t *testing.T) {
		// This assertion is tricky. We want to ensure NO code was generated for ResourceEdges.
		// A simple check is that the generated file should not contain 'ResourceEdges'.
		// The ideal fix would be to make the generated code valid Go code.
		if strings.Contains(generatedCode, "ResourceEdges") {
			t.Errorf("Generated code still contains 'ResourceEdges', expected it to be skipped. Content:\n%s", generatedCode)
		}

		// Further validation: try to parse the generated code to catch compilation errors
		fset := token.NewFileSet()
		_, parseErr := parser.ParseFile(fset, "", generatedCode, 0)
		if parseErr != nil {
			t.Errorf("Generated code is not valid Go code (due to undefined types?): %v\nContent:\n%s", parseErr, generatedCode)
		}
	})

	// Add more specific assertions here if needed as the bug becomes clearer
}
