package core

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	testing "testing"
)

func TestGenerator_EndToEnd(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	g := NewGenerator()
	tempDir := t.TempDir()
	g.Output = tempDir

	sourceDir, err := filepath.Abs("../ast/testdata")
	if err != nil {
		t.Fatalf("Failed to get absolute path for source dir: %v", err)
	}

	if err := g.ParseSource(sourceDir); err != nil {
		t.Fatalf("ParseSource() failed: %v", err)
	}

	if err := g.Generate(); err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	generatedFile := filepath.Join(tempDir, "testdata.gen.go")
	content, err := os.ReadFile(generatedFile)
	if err != nil {
		t.Fatalf("Failed to read generated file: %v", err)
	}
	
generatedCode := string(content)
	t.Logf("Generated Code:\n%s", generatedCode)

	t.Run("GeneratedCodeValidation", func(t *testing.T) {
		// Check package and imports
		if !strings.Contains(generatedCode, "package testdata") {
			t.Error("expected package 'testdata'")
		}
		if !strings.Contains(generatedCode, `ent "github.com/origadmin/abgen/internal/testdata/ent"`) {
			t.Error("expected import for 'ent' package")
		}
		if !strings.Contains(generatedCode, `typespb "github.com/origadmin/abgen/internal/testdata/typespb"`) {
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
