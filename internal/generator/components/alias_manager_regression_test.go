package components

import (
	"testing"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

// TestAliasManager_PointerVsValue_NoCollision ensures that a type T and its pointer *T
// do not generate conflicting or incorrect aliases. The pointer type itself should never get an alias.
func TestAliasManager_PointerVsValue_NoCollision(t *testing.T) {
	deptType := newStruct("Department", "source/ent", nil)
	pointerDeptType := newPointer(deptType)

	analysisResult := &model.AnalysisResult{
		TypeInfos: map[string]*model.TypeInfo{
			"source/ent.Department":  deptType,
			"*source/ent.Department": pointerDeptType,
		},
		ExistingAliases: make(map[string]string),
		ExecutionPlan: &model.ExecutionPlan{
			FinalConfig: config.NewConfig(),
		},
	}
	cfg := analysisResult.ExecutionPlan.FinalConfig
	// This rule is enough to trigger processing for source/ent.Department
	cfg.ConversionRules = append(cfg.ConversionRules, &config.ConversionRule{
		SourceType: "source/ent.Department",
		TargetType: "target/dto.Department",
	})
	cfg.NamingRules.SourceSuffix = "Source"

	im := &mockImportManager{aliases: make(map[string]string)}
	am := NewAliasManager(analysisResult, im)

	// Call the public API. This should process all types reachable from the conversion rules.
	am.PopulateAliases()

	// --- Assertions ---
	valueAlias, okValue := am.LookupAlias("source/ent.Department")
	_, okPointer := am.LookupAlias("*source/ent.Department")

	if !okValue {
		t.Fatal("Alias for value type 'Department' was not generated, but it should have been.")
	}
	if okPointer {
		t.Error("Alias for pointer type '*Department' was generated, but it should NOT have been.")
	}
	if valueAlias != "DepartmentSource" {
		t.Errorf("Expected value alias to be 'DepartmentSource', got '%s'", valueAlias)
	}
}

// TestAliasManager_ThirdPartyTypes_NotAliased ensures that types from packages that are not
// part of the source or target packages (e.g., timestamppb) are never aliased.
func TestAliasManager_ThirdPartyTypes_NotAliased(t *testing.T) {
	timestampType := newStruct("Timestamp", "google.golang.org/protobuf/types/known/timestamppb", nil)
	eventType := newStruct("Event", "source/ent", []*model.FieldInfo{
		{Name: "OccurredAt", Type: newPointer(timestampType)},
	})

	analysisResult := &model.AnalysisResult{
		TypeInfos: map[string]*model.TypeInfo{
			"source/ent.Event": eventType,
			"google.golang.org/protobuf/types/known/timestamppb.Timestamp": timestampType,
		},
		ExistingAliases: make(map[string]string),
		ExecutionPlan: &model.ExecutionPlan{
			FinalConfig: config.NewConfig(),
		},
	}
	cfg := analysisResult.ExecutionPlan.FinalConfig
	cfg.ConversionRules = append(cfg.ConversionRules, &config.ConversionRule{
		SourceType: "source/ent.Event",
		TargetType: "target/dto.Event",
	})
	cfg.NamingRules.SourceSuffix = "Source"

	im := &mockImportManager{aliases: make(map[string]string)}
	am := NewAliasManager(analysisResult, im)
	am.PopulateAliases()

	// --- Assertions ---
	eventAlias, okEvent := am.LookupAlias("source/ent.Event")
	_, okTimestamp := am.LookupAlias("google.golang.org/protobuf/types/known/timestamppb.Timestamp")

	if !okEvent {
		t.Fatal("Alias for managed type 'Event' was not generated.")
	}
	if eventAlias != "EventSource" {
		t.Errorf("Expected alias for 'Event' to be 'EventSource', got '%s'", eventAlias)
	}
	if okTimestamp {
		t.Error("Alias for third-party type 'timestamppb.Timestamp' was generated, but it should not have been.")
	}
}

// TestAliasManager_PluralizationWithSuffix ensures that pluralization ('s') is applied
// to the base name BEFORE a suffix is added.
func TestAliasManager_PluralizationWithSuffix(t *testing.T) {
	userType := newStruct("User", "source/ent", nil)
	userSliceType := newSlice(userType)

	analysisResult := &model.AnalysisResult{
		TypeInfos: map[string]*model.TypeInfo{
			"source/ent.User":   userType,
			"[]source/ent.User": userSliceType,
		},
		ExistingAliases: make(map[string]string),
		ExecutionPlan: &model.ExecutionPlan{
			FinalConfig: config.NewConfig(),
		},
	}
	cfg := analysisResult.ExecutionPlan.FinalConfig
	// Add rules for both singular and plural types to ensure they are processed.
	cfg.ConversionRules = append(cfg.ConversionRules, &config.ConversionRule{
		SourceType: "source/ent.User",
		TargetType: "target/dto.User",
	})
	cfg.ConversionRules = append(cfg.ConversionRules, &config.ConversionRule{
		SourceType: "[]source/ent.User",
		TargetType: "[]target/dto.User",
	})
	cfg.NamingRules.TargetSuffix = "PB"

	im := &mockImportManager{aliases: make(map[string]string)}
	am := NewAliasManager(analysisResult, im)

	am.PopulateAliases()

	// --- Assertions ---
	userAlias, _ := am.LookupAlias("source/ent.User")
	sliceAlias, _ := am.LookupAlias("[]source/ent.User")

	expectedUserAlias := "UserPB"
	expectedSliceAlias := "UsersPB" // Not "UserPBs"

	if userAlias != expectedUserAlias {
		t.Errorf("Expected singular alias to be '%s', got '%s'", expectedUserAlias, userAlias)
	}
	if sliceAlias != expectedSliceAlias {
		t.Errorf("Expected slice alias to be '%s', got '%s'", expectedSliceAlias, sliceAlias)
	}
}
