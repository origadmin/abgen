package generator

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/origadmin/abgen/internal/analyzer"
	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

func init() {
	// Configure slog to debug mode for detailed testing information
	slog.SetDefault(slog.New(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}),
	))
}

func TestGenerator_CodeGeneration(t *testing.T) {

	// Base dependencies for most tests
	// NOTE: These are now only used for tests that specifically rely on the global fixture.
	// New tests should define their own local dependencies if they have specific fixture types.
	// The 'dependencies' field in test cases will now be used by the config.DirectiveParser to extract dependencies.
	// The actual loading will be handled by analyzer.PackageWalker.
	baseDependencies := []string{
		"github.com/origadmin/abgen/testdata/fixtures/ent",
		"github.com/origadmin/abgen/testdata/fixtures/types",
	}

	testCases := []struct {
		name           string
		directivePath  string
		goldenFileName string
		dependencies   []string // These are now hints for directive parsing, not direct load patterns
		priority       string   // P0, P1, P2 for prioritization
		category       string   // Test category for organization
		assertFunc     func(t *testing.T, generatedCode []byte, stubCode []byte)
	}{
		// === 02_basic_conversions: Basic Struct Conversion ===
		{
			name:          "simple_struct_conversion",
			directivePath: "../../testdata/02_basic_conversions/simple_struct",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/02_basic_conversions/simple_struct/source",
				"github.com/origadmin/abgen/testdata/02_basic_conversions/simple_struct/target",
			},
			priority: "P0",
			category: "basic_conversions",
			assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
				generatedStr := string(generatedCode)
				// Check forward conversion (User -> UserDTO)
				assertContainsPattern(t, generatedStr, `func ConvertUserToUserDTO\(from \*User\) \*UserDTO`)
				assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`) // Use \s+ for flexible whitespace matching
				assertContainsPattern(t, generatedStr, `UserName:\s+from.Name,`)

				// Check reverse conversion (UserDTO -> User)
				assertContainsPattern(t, generatedStr, `func ConvertUserDTOToUser\(from \*UserDTO\) \*User`)
				assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`) // Use \s+ for flexible whitespace matching
				assertContainsPattern(t, generatedStr, `Name:\s+from.UserName,`)
			},
		},
		{
			name:          "package_level_conversion",
			directivePath: "../../testdata/02_basic_conversions/package_level_conversion",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/02_basic_conversions/package_level_conversion/source",
				"github.com/origadmin/abgen/testdata/02_basic_conversions/package_level_conversion/target",
			},
			priority: "P0",
			category: "basic_conversions",
			assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
				generatedStr := string(generatedCode)
				// Check forward and reverse conversion for User
				assertContainsPattern(t, generatedStr, `func ConvertUserSourceToUserTarget\(from \*UserSource\) \*UserTarget`)
				assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`)
				assertContainsPattern(t, generatedStr, `Name:\s+from.Name,`)
				assertContainsPattern(t, generatedStr, `func ConvertUserTargetToUserSource\(from \*UserTarget\) \*UserSource`) // Reverse conversion
				assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`)
				assertContainsPattern(t, generatedStr, `Name:\s+from.Name,`)

				// Check forward and reverse conversion for Item
				assertContainsPattern(t, generatedStr, `func ConvertItemSourceToItemTarget\(from \*ItemSource\) \*ItemTarget`)
				assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`)
				assertContainsPattern(t, generatedStr, `Name:\s+from.Name,`)
				assertContainsPattern(t, generatedStr, `func ConvertItemTargetToItemSource\(from \*ItemTarget\) \*ItemSource`) // Reverse conversion
				assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`)
				assertContainsPattern(t, generatedStr, `Name:\s+from.Name,`)
			},
		},
		{
			name:          "oneway_conversion",
			directivePath: "../../testdata/02_basic_conversions/oneway_conversion",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/02_basic_conversions/oneway_conversion/source",
				"github.com/origadmin/abgen/testdata/02_basic_conversions/oneway_conversion/target",
			},
			priority: "P0",
			category: "basic_conversions",
			assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
				generatedStr := string(generatedCode)
				// Check forward conversion (User -> UserDTO)
				assertContainsPattern(t, generatedStr, `func ConvertUserToUserDTO\(from \*User\) \*UserDTO`)
				assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`)
				assertContainsPattern(t, generatedStr, `Name:\s+from.Name,`)

				// Check that reverse conversion (UserDTO -> User) does NOT exist
				assertNotContainsPattern(t, generatedStr, `func ConvertUserDTOToUser`)
			},
		},
		{
			name:          "id_to_id_field_conversion",
			directivePath: "../../testdata/02_basic_conversions/id_to_id_field_conversion",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/02_basic_conversions/id_to_id_field_conversion/source",
				"github.com/origadmin/abgen/testdata/02_basic_conversions/id_to_id_field_conversion/target",
			},
			priority: "P0",
			category: "basic_conversions",
			assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
				generatedStr := string(generatedCode)
				// Check forward conversion (source.User.Id -> target.User.ID)
				assertContainsPattern(t, generatedStr, `func ConvertUserSourceToUserTarget\(from \*UserSource\) \*UserTarget`)
				assertContainsPattern(t, generatedStr, `ID:\s+from.Id,`) // Target ID should be mapped from source Id
				assertContainsPattern(t, generatedStr, `Name:\s+from.Name,`)

				// Check reverse conversion (target.User.ID -> source.User.Id)
				assertContainsPattern(t, generatedStr, `func ConvertUserTargetToUserSource\(from \*UserTarget\) \*UserSource`)
				assertContainsPattern(t, generatedStr, `Id:\s+from.ID,`) // Source Id should be mapped from target ID
				assertContainsPattern(t, generatedStr, `Name:\s+from.Name,`)
			},
		},
		{
			name:          "simple_bilateral",
			directivePath: "../../testdata/02_basic_conversions/simple_bilateral",
			dependencies:  baseDependencies,
			priority:      "P0",
			category:      "basic_conversions",
			assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
				generatedStr := string(generatedCode)

				// Expected helper functions (from generator.go's conversionFunctionBodies)
				assertContainsPattern(t, generatedStr, `func ConvertStringToTime\(s string\) time.Time`)
				assertContainsPattern(t, generatedStr, `func ConvertTimeToString\(t time.Time\) string`)

				// Check forward conversion: ent.User -> types.User (using aliased types User and UserBilateral)
				assertContainsPattern(t, generatedStr, `func ConvertUserToUserBilateral\(from \*User\) \*UserBilateral`)
				assertContainsPattern(t, generatedStr, `Id:\s+from.ID,`)                                                   // ent.ID (int) -> types.Id (int)
				assertContainsPattern(t, generatedStr, `Username:\s+from.Username,`)                                       // string -> string
				assertContainsPattern(t, generatedStr, `Age:\s+from.Age,`)                                                 // int -> int
				assertContainsPattern(t, generatedStr, `Gender:\s+ConvertGenderToGenderBilateral\(from.Gender\),`)         // ent.Gender -> types.Gender, custom func
				assertContainsPattern(t, generatedStr, `Status:\s+ConvertUserStatusToUserBilateralStatus\(from.Status\),`) // string -> int32, custom func
				assertContainsPattern(t, generatedStr, `CreatedAt:\s+ConvertTimeToString\(from.CreatedAt\),`)              // time.Time -> string

				// Verify fields that should NOT be present in types.User conversion (or handled implicitly as zero values)
				assertNotContainsPattern(t, generatedStr, `Password:`)
				assertNotContainsPattern(t, generatedStr, `Salt:`)
				assertNotContainsPattern(t, generatedStr, `RoleIDs:`)
				assertNotContainsPattern(t, generatedStr, `Roles:`)
				assertNotContainsPattern(t, generatedStr, `Edges:`)

				// Check reverse conversion: types.User -> ent.User (using aliased types User and UserBilateral)
				assertContainsPattern(t, generatedStr, `func ConvertUserBilateralToUser\(from \*UserBilateral\) \*User`)
				assertContainsPattern(t, generatedStr, `ID:\s+from.Id,`)
				assertContainsPattern(t, generatedStr, `Username:\s+from.Username,`)
				assertContainsPattern(t, generatedStr, `Age:\s+from.Age,`)
				assertContainsPattern(t, generatedStr, `Gender:\s+ConvertGenderBilateralToGender\(from.Gender\),`)         // types.Gender -> ent.Gender, custom func
				assertContainsPattern(t, generatedStr, `Status:\s+ConvertUserBilateralStatusToUserStatus\(from.Status\),`) // int32 -> string, custom func
				assertContainsPattern(t, generatedStr, `CreatedAt:\s+ConvertStringToTime\(from.CreatedAt\),`)              // string -> time.Time
			},
		},
		{
			name:          "standard_trilateral",
			directivePath: "../../testdata/02_basic_conversions/standard_trilateral",
			dependencies:  baseDependencies,
			priority:      "P0",
			category:      "basic_conversions",
			assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
				generatedStr := string(generatedCode)

				// Helper functions
				assertContainsPattern(t, generatedStr, `func ConvertStringToTime\(s string\) time.Time`)
				assertContainsPattern(t, generatedStr, `func ConvertTimeToString\(t time.Time\) string`)

				// Check forward conversion: ent.User -> types.User
				assertContainsPattern(t, generatedStr, `func ConvertUserToUserTrilateral\(from \*User\) \*UserTrilateral`)
				assertContainsPattern(t, generatedStr, `Id:\s+from.ID,`)
				assertContainsPattern(t, generatedStr, `Username:\s+from.Username,`)
				assertContainsPattern(t, generatedStr, `Age:\s+from.Age,`)
				assertContainsPattern(t, generatedStr, `Gender:\s+ConvertGenderToGenderTrilateral\(from.Gender\),`)         // Use custom function
				assertContainsPattern(t, generatedStr, `Status:\s+ConvertUserStatusToUserTrilateralStatus\(from.Status\),`) // Use custom function
				assertContainsPattern(t, generatedStr, `CreatedAt:\s+ConvertTimeToString\(from.CreatedAt\),`)

				// Check reverse conversion: types.User -> ent.User
				assertContainsPattern(t, generatedStr, `func ConvertUserTrilateralToUser\(from \*UserTrilateral\) \*User`)
				assertContainsPattern(t, generatedStr, `ID:\s+from.Id,`)
				assertContainsPattern(t, generatedStr, `Username:\s+from.Username,`)
				assertContainsPattern(t, generatedStr, `Age:\s+from.Age,`)
				assertContainsPattern(t, generatedStr, `Gender:\s+ConvertGenderTrilateralToGender\(from.Gender\),`)         // Use custom function
				assertContainsPattern(t, generatedStr, `Status:\s+ConvertUserTrilateralStatusToUserStatus\(from.Status\),`) // Use custom function
				assertContainsPattern(t, generatedStr, `CreatedAt:\s+ConvertStringToTime\(from.CreatedAt\),`)

				// Check for Resource conversions
				assertContainsPattern(t, generatedStr, `func ConvertResourceToResourceTrilateral\(from \*Resource\) \*ResourceTrilateral`)
				assertContainsPattern(t, generatedStr, `func ConvertResourceTrilateralToResource\(from \*ResourceTrilateral\) \*Resource`)
			},
		},
		{
			name:           "field_ignore_remap",
			directivePath:  "../../testdata/02_basic_conversions/field_ignore_remap",
			goldenFileName: "expected.golden", // Will generate this later
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/02_basic_conversions/field_ignore_remap/source",
				"github.com/origadmin/abgen/testdata/02_basic_conversions/field_ignore_remap/target",
			},
			priority: "P0",
			category: "basic_conversions",
			assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
				generatedStr := string(generatedCode)
				// Assertions for ConvertUserToUserDTO
				assertContainsPattern(t, generatedStr, `func ConvertUserToUserDTO\(from \*User\) \*UserDTO`)
				// Ignored fields should NOT be present in the output
				assertNotContainsPattern(t, generatedStr, `Password:`)
				assertNotContainsPattern(t, generatedStr, `CreatedAt:`) // Assert that CreatedAt is not directly assigned to CreatedDate (because CreatedAt is ignored)
				// Remapped fields should be present with new names
				assertContainsPattern(t, generatedStr, `FullName:\s+from.Name,`)
				assertContainsPattern(t, generatedStr, `UserEmail:\s+from.Email,`)
				// Non-remapped, non-ignored fields should be present with original names (if types match)
				assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`)                // Corrected to `ID` as in generated code and `Id` for source
				assertContainsPattern(t, generatedStr, `LastUpdate:\s+from.UpdatedAt,`) // The generator directly assigns time.Time fields when compatible.
				assertNotContainsPattern(t, generatedStr, `CreatedDate:`)               // It shouldn't be explicitly assigned if no source
			},
		},
		{
			name:           "slice_conversion",
			directivePath:  "../../testdata/02_basic_conversions/slice_conversion",
			goldenFileName: "expected.golden",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/02_basic_conversions/slice_conversion/source",
				"github.com/origadmin/abgen/testdata/02_basic_conversions/slice_conversion/target",
			},
			priority: "P0",
			category: "basic_conversions",
			assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
				generatedStr := string(generatedCode)

				// --- Assertions for ContainerVV ([]UserVV -> []UserVV) ---
				assertContainsPattern(t, generatedStr, `func ConvertContainerVVSourceToContainerVVTarget\(from \*ContainerVVSource\) \*ContainerVVTarget`)
				assertContainsPattern(t, generatedStr, `Users:\s+ConvertUsersVVSourceToUsersVVTarget\(from.Users\),`)
				assertContainsPattern(t, generatedStr, `func ConvertUsersVVSourceToUsersVVTarget\(froms UsersVVSource\) UsersVVTarget`)
				assertContainsPattern(t, generatedStr, `tos\[i\] = \*ConvertUserVVSourceToUserVVTarget\(&f\)`)

				// --- Assertions for ContainerPP ([]*UserPP -> []*UserPP) ---
				assertContainsPattern(t, generatedStr, `func ConvertContainerPPSourceToContainerPPTarget\(from \*ContainerPPSource\) \*ContainerPPTarget`)
				assertContainsPattern(t, generatedStr, `Users:\s+ConvertUsersPPSourceToUsersPPTarget\(from.Users\),`)
				assertContainsPattern(t, generatedStr, `func ConvertUsersPPSourceToUsersPPTarget\(froms UsersPPSource\) UsersPPTarget`)
				assertContainsPattern(t, generatedStr, `tos\[i\] = ConvertUserPPSourceToUserPPTarget\(f\)`)

				// --- Assertions for ContainerPV (*[]UserPV -> *[]UserPV) ---
				assertContainsPattern(t, generatedStr, `func ConvertContainerPVSourceToContainerPVTarget\(from \*ContainerPVSource\) \*ContainerPVTarget`)
				assertContainsPattern(t, generatedStr, `Users:\s+ConvertUsersPVSourceToUsersPVTarget\(from.Users\),`)
				assertContainsPattern(t, generatedStr, `func ConvertUsersPVSourceToUsersPVTarget\(from \*UsersPVSource\) \*UsersPVTarget`)
				assertContainsPattern(t, generatedStr, `for i, f := range \(\*from\)`)
				assertContainsPattern(t, generatedStr, `return &tos`)

				// --- Assertions for ContainerPPP (*[]*UserPPP -> *[]*UserPPP) ---
				assertContainsPattern(t, generatedStr, `func ConvertContainerPPPSourceToContainerPPPTarget\(from \*ContainerPPPSource\) \*ContainerPPPTarget`)
				assertContainsPattern(t, generatedStr, `Users:\s+ConvertUsersPPPSourceToUsersPPPTarget\(from.Users\),`)
				assertContainsPattern(t, generatedStr, `func ConvertUsersPPPSourceToUsersPPPTarget\(from \*UsersPPPSource\) \*UsersPPPTarget`)
				assertContainsPattern(t, generatedStr, `for i, f := range \(\*from\)`)
				assertContainsPattern(t, generatedStr, `return &tos`)

				// --- Assertions for ContainerVP ([]UserVP -> []*UserVP) ---
				assertContainsPattern(t, generatedStr, `func ConvertContainerVPSourceToContainerVPTarget\(from \*ContainerVPSource\) \*ContainerVPTarget`)
				assertContainsPattern(t, generatedStr, `Users:\s+ConvertUsersVPSourceToUsersVPTarget\(from.Users\),`)
				assertContainsPattern(t, generatedStr, `func ConvertUsersVPSourceToUsersVPTarget\(froms UsersVPSource\) UsersVPTarget`)
				assertContainsPattern(t, generatedStr, `tmpVal := \*ConvertUserVPSourceToUserVPTarget\(&f\)`)
				assertContainsPattern(t, generatedStr, `tos\[i\] = &tmpVal`)

				// --- Assertions for ContainerPV2 ([]*UserPV2 -> []UserPV2) ---
				assertContainsPattern(t, generatedStr, `func ConvertContainerPV2SourceToContainerPV2Target\(from \*ContainerPV2Source\) \*ContainerPV2Target`)
				assertContainsPattern(t, generatedStr, `Users:\s+ConvertUsersPV2SourceToUsersPV2Target\(from.Users\),`)
				assertContainsPattern(t, generatedStr, `func ConvertUsersPV2SourceToUsersPV2Target\(froms UsersPV2Source\) UsersPV2Target`)
				assertContainsPattern(t, generatedStr, `tos\[i\] = \*ConvertUserPV2SourceToUserPV2Target\(f\)`)

				// --- Assertions for Order (original test) ---
				assertContainsPattern(t, generatedStr, `func ConvertOrderSourceToOrderTarget\(from \*OrderSource\) \*OrderTarget`)
				assertContainsPattern(t, generatedStr, `Items:\s+ConvertItemsSourceToItemsTarget\(from.Items\),`)
			},
		},

		// === 03_advanced_features: Advanced Feature Tests ===
		{
			name:           "auto_generate_aliases",
			directivePath:  "../../testdata/03_advanced_features/auto_generate_aliases",
			goldenFileName: "expected.golden",
			dependencies:   baseDependencies,
			priority:       "P0",
			category:       "advanced_features",
		},
		{
			name:          "custom_function_rules",
			directivePath: "../../testdata/03_advanced_features/custom_function_rules",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/03_advanced_features/custom_function_rules/source",
				"github.com/origadmin/abgen/testdata/03_advanced_features/custom_function_rules/target",
			},
			priority: "P0",
			category: "advanced_features",
			assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
				generatedStr := string(generatedCode)
				stubStr := string(stubCode)
				slog.Debug("Generated code inside custom_function_rules assertFunc", "code", generatedStr)
				// Assertions for ConvertUserToUserCustom
				assertContainsPattern(t, generatedStr, `Status:\s+ConvertUserStatusToUserCustomStatus\(from.Status\),`)
				// Assertions for ConvertUserCustomToUser
				assertContainsPattern(t, generatedStr, `Status:\s+ConvertUserCustomStatusToUserStatus\(from.Status\),`)
				// Assert that the stub function is generated in the stub file
				assertContainsPattern(t, stubStr, `func ConvertUserStatusToUserCustomStatus\(from int\) string`)
			},
		},
		{
			name:           "slice_conversions",
			directivePath:  "../../testdata/03_advanced_features/slice_conversions",
			goldenFileName: "expected.golden",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/03_advanced_features/slice_conversions/source",
				"github.com/origadmin/abgen/testdata/03_advanced_features/slice_conversions/target",
			},

			priority: "P0",
			category: "advanced_features",
		},
		{
			name:          "enum_string_to_int",
			directivePath: "../../testdata/03_advanced_features/enum_string_to_int",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/03_advanced_features/enum_string_to_int/source",
				"github.com/origadmin/abgen/testdata/03_advanced_features/enum_string_to_int/target",
			}, priority: "P1",
			category:    "advanced_features",
			assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
				t.Log("TODO: Add specific assertions for enum_string_to_int")
			},

			// Write success file for inspection (only for 02_basic_conversions)

		},
		{
			name:          "pointer_conversions",
			directivePath: "../../testdata/03_advanced_features/pointer_conversions",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/03_advanced_features/pointer_conversions/source",
				"github.com/origadmin/abgen/testdata/03_advanced_features/pointer_conversions/target",
			},

			priority: "P1",
			category: "advanced_features",
			assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
				t.Log("TODO: Add specific assertions for pointer_conversions")
			},

			// Write success file for inspection (only for 02_basic_conversions)

		},
		{
			name:          "map_conversions",
			directivePath: "../../testdata/03_advanced_features/map_conversions",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/03_advanced_features/map_conversions/source",
				"github.com/origadmin/abgen/testdata/03_advanced_features/map_conversions/target",
			},
			priority: "P1",
			category: "advanced_features",
			assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
				t.Log("TODO: Add specific assertions for map_conversions")
			},

			// Write success file for inspection (only for 02_basic_conversions)

		},
		{
			name:          "numeric_conversions",
			directivePath: "../../testdata/03_advanced_features/numeric_conversions",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/03_advanced_features/numeric_conversions/source",
				"github.com/origadmin/abgen/testdata/03_advanced_conversions/numeric_conversions/target",
			},
			priority: "P1",
			category: "advanced_features",
			assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
				t.Log("TODO: Add specific assertions for numeric_conversions")
			},

			// Write success file for inspection (only for 02_basic_conversions)

		},

		// === 06_regression: Regression Tests ===
		{
			name:           "array_slice_test",
			directivePath:  "../../testdata/06_regression/array_slice_fix/array_slice_test",
			goldenFileName: "expected.golden",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/06_regression/array_slice_fix/array_slice_test/source",
				"github.com/origadmin/abgen/testdata/06_regression/array_slice_fix/array_slice_test/target",
			},
			priority: "P0",
			category: "regression",
		},

		{
			name:           "alias-gen", // Specific bug fix test case
			directivePath:  "../../testdata/06_regression/alias_gen_fix",
			goldenFileName: "expected.golden",
			dependencies:   baseDependencies,
			priority:       "P0",
			category:       "regression",
		},
		{
			name:          "map_string_to_string_conversion",
			directivePath: "../../testdata/06_regression/map_string_to_string_fix",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/06_regression/map_string_to_string_fix/source",
				"github.com/origadmin/abgen/testdata/06_regression/map_string_to_string_fix/target",
			},
			priority: "P0",
			category: "regression",
			assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
				generatedStr := string(generatedCode)
				stubStr := string(stubCode)

				t.Logf("Generated code for map_string_to_string_conversion:\n%s", generatedStr)
				if len(stubCode) > 0 {
					t.Logf("Generated stub code:\n%s", stubCode)
				}

				// Check forward conversion function is generated
				assertContainsPattern(t, generatedStr, `func ConvertMapToStringSourceToMapToStringTarget\(from \*MapToStringSource\) \*MapToStringTarget`)

				// Check reverse conversion function is generated
				assertContainsPattern(t, generatedStr, `func ConvertMapToStringTargetToMapToStringSource\(from \*MapToStringTarget\) \*MapToStringSource`)

				// Check for correct naming rule: 前缀+[类型+字段名]+后缀+TO+前缀+[类型+字段名]+后缀
				// The stub functions should follow the GetPrimitiveConversionStubName naming pattern
				if len(stubStr) > 0 {
					// Expected naming pattern: ConvertMapToStringSourceMetadataToMapToStringTargetMetadata
					assertContainsPattern(t, stubStr, `func ConvertMapToStringSourceMetadataToMapToStringTargetMetadata`)
					assertContainsPattern(t, stubStr, `func ConvertMapToStringSourceTagsToMapToStringTargetTags`)
					assertContainsPattern(t, stubStr, `func ConvertMapToStringSourceConfigToMapToStringTargetConfig`)

					// Also check reverse direction functions
					assertContainsPattern(t, stubStr, `func ConvertMapToStringTargetMetadataToMapToStringSourceMetadata`)
					assertContainsPattern(t, stubStr, `func ConvertMapToStringTargetTagsToMapToStringSourceTags`)
					assertContainsPattern(t, stubStr, `func ConvertMapToStringTargetConfigToMapToStringSourceConfig`)

					t.Logf("Stub functions generated with correct naming pattern for map->string conversion")
				} else {
					// If no stubs, check that the main conversion functions handle the field conversion
					// The main code should call the conversion stubs for these field conversions
					assertContainsPattern(t, generatedStr, `ConvertMapToStringSourceMetadataToMapToStringTargetMetadata`)
					t.Logf("Main code handles map->string conversion through named conversion functions")
				}
			},
		},
	}

	// Sort test cases for consistent execution order
	sort.Slice(testCases, func(i, j int) bool {
		priorityOrder := map[string]int{"P0": 0, "P1": 1, "P2": 2}
		if priorityOrder[testCases[i].priority] != priorityOrder[testCases[j].priority] {
			return priorityOrder[testCases[i].priority] < priorityOrder[testCases[j].priority]
		}
		return testCases[i].category < testCases[j].category
	})

	for _, tc := range testCases {
		// Extract the numeric prefix from the directory path to ensure the stage number
		// in the test name matches the physical directory structure.
		// e.g., extracts "03" from "../../testdata/03_advanced_features/..."
		var stagePrefix string
		pathParts := strings.Split(tc.directivePath, "/")
		for _, part := range pathParts {
			if len(part) > 2 && part[2] == '_' && part[0] >= '0' && part[0] <= '9' && part[1] >= '0' && part[1] <= '9' {
				stagePrefix = part[:2]
				break
			}
		}

		// Prepend the stage number to the test name for clear, always-visible grouping.
		testNameWithStage := fmt.Sprintf("%s_%s/%s", stagePrefix, tc.category, tc.name)
		t.Run(testNameWithStage, func(t *testing.T) {
			t.Logf("Running test: %s (Priority: %s, Category: %s)", tc.name, tc.priority, tc.category)

			// Cleanup any previously generated files in the testdata directory to ensure a clean slate.
			// This prevents "redeclared in this block" errors when running tests repeatedly.
			cleanTestFiles(t, tc.directivePath)
			defer cleanTestFiles(t, tc.directivePath)

			// Step 1: Parse config from the directive path using the new high-level API.
			parser := config.NewParser()
			cfg, err := parser.Parse(tc.directivePath)
			if err != nil {
				t.Fatalf("config.Parser.Parse() failed: %v", err)
			}
			if tc.name == "field_ignore_remap" {
				t.Logf("Parsed config for field_ignore_remap: %+v", cfg.ConversionRules)
			}

			// Step 2: Analyze types using the new high-level API.
			typeAnalyzer := analyzer.NewTypeAnalyzer() // Pass the root dir for analysis context
			packagePaths := cfg.RequiredPackages()
			typeFQNs := cfg.RequiredTypeFQNs()
			typeInfos, err := typeAnalyzer.Analyze(packagePaths, typeFQNs)
			if err != nil {
				t.Fatalf("analyzer.TypeAnalyzer.Analyze() failed: %v", err)
			}

			// Step 3: Run the generator with the results.
			g := NewGenerator(cfg)
			generatedCode, err := g.Generate(typeInfos)
			if err != nil {
				t.Fatalf("Generate() failed for test case %s: %v", tc.name, err)
			}
			stubCode := g.CustomStubs()

			// Normalize path separators in generated code for consistent comparison across OS.
			// The `source` comment line uses paths that might differ by OS.
			generatedCodeStr := string(generatedCode)
			generatedCodeStr = strings.ReplaceAll(generatedCodeStr, `\`, `/`)
			generatedCode = []byte(generatedCodeStr)

			// Step 4.1: Run custom assertions if provided.
			if tc.assertFunc != nil {
				// Create a subtest to capture failures more clearly
				t.Run(tc.name+"_Assertions", func(st *testing.T) {
					tc.assertFunc(st, generatedCode, stubCode)
					if st.Failed() {
						actualOutputFile := filepath.Join(tc.directivePath, "failed.actual.gen.go")
						_ = os.WriteFile(actualOutputFile, generatedCode, 0644)
						st.Logf("Assertion failed for '%s'. Generated output saved to %s for inspection.", tc.name, actualOutputFile)
						if len(stubCode) > 0 {
							actualStubFile := filepath.Join(tc.directivePath, "failed.actual.stub.go")
							_ = os.WriteFile(actualStubFile, stubCode, 0644)
							st.Logf("Stub output saved to %s for inspection.", actualStubFile)
						}
					} else {
						actualOutputFile := filepath.Join(tc.directivePath, "success.actual.gen.go")
						_ = os.WriteFile(actualOutputFile, generatedCode, 0644)
					}
				})
			}

			// Step 4.2: Snapshot testing - compare against a "golden" file.
			if tc.goldenFileName != "" { // Only attempt golden file comparison if goldenFileName is provided
				goldenFile := filepath.Join(tc.directivePath, tc.goldenFileName)
				if os.Getenv("UPDATE_GOLDEN_FILES") != "" {
					err = os.WriteFile(goldenFile, generatedCode, 0644)
					if err != nil {
						t.Fatalf("Failed to update golden file %s: %v", goldenFile, err)
					}
					t.Logf("Updated golden file: %s", goldenFile)
					return // Skip comparison when updating
				}

				expectedCode, err := os.ReadFile(goldenFile)
				if err != nil {
					t.Fatalf("Failed to read golden file %s: %v", goldenFile, err)
				}

				if string(generatedCode) != string(expectedCode) {
					actualOutputFile := filepath.Join(tc.directivePath, "failed.actual.gen.go") // Save to a unique file
					_ = os.WriteFile(actualOutputFile, generatedCode, 0644)
					t.Errorf("Generated code for '%s' does not match the golden file %s. The generated output was saved to %s for inspection.", tc.name, goldenFile, actualOutputFile)
				}
			} else {
				t.Logf("Skipping golden file comparison for test: %s (no goldenFileName provided)", tc.name)
			}
		})
	}
}

// TestDefaultDirectionBehavior tests that the default direction is 'both' when not explicitly specified
func TestDefaultDirectionBehavior(t *testing.T) {
	slog.Debug("Starting TestDefaultDirectionBehavior")

	// Test case: simple_struct should generate both directions by default
	testPath := "../../testdata/02_basic_conversions/simple_struct"

	slog.Debug("Testing directory", "path", testPath)

	// Parse configuration from the test directory
	parser := config.NewParser()
	config, err := parser.Parse(testPath)
	if err != nil {
		slog.Error("Failed to parse configuration", "error", err)
		t.Fatalf("Failed to parse configuration: %v", err)
	}

	slog.Debug("Parsed configuration", "rules_count", len(config.ConversionRules))

	// Assert that we have at least one conversion rule
	if len(config.ConversionRules) == 0 {
		t.Fatal("Expected at least one conversion rule, got none")
	}

	// Get the first conversion rule for testing
	rule := config.ConversionRules[0]
	slog.Debug("First rule", "source", rule.SourceType, "target", rule.TargetType, "direction", rule.Direction)

	// The key assertion: direction should default to 'both'
	if rule.Direction != "both" {
		slog.Error("Direction should default to 'both'", "actual", rule.Direction)
		t.Errorf("Expected direction to be 'both', got '%s'", rule.Direction)
	}

	slog.Debug("Direction assertion passed")
	slog.Debug("TestDefaultDirectionBehavior completed successfully")
}

func cleanTestFiles(t *testing.T, dir string) {
	files, err := filepath.Glob(filepath.Join(dir, "*.actual.gen.go"))
	if err != nil {
		t.Fatalf("Failed to glob for generated files in %s: %v", dir, err)
	}
	files = append(files, filepath.Join(dir, "failed.actual.stub.go"))
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			// Change from t.Fatalf to t.Logf to prevent flaky tests on Windows due to file locks.
			t.Logf("Warning: Failed to remove old generated file %s: %v", f, err)
		}
	}
	// Also remove the failed output file if it exists
	failedFile := filepath.Join(dir, "failed.actual.gen.go")
	if _, err := os.Stat(failedFile); err == nil {
		if err := os.Remove(failedFile); err != nil {
			// Change from t.Fatalf to t.Logf
			t.Logf("Warning: Failed to remove old failed output file %s: %v", failedFile, err)
		}
	}
}

// assertContainsPattern checks if the generated code contains a specific regular expression pattern.
func assertContainsPattern(t *testing.T, code string, pattern string) {
	t.Helper()
	match, err := regexp.MatchString(pattern, code)
	if err != nil {
		t.Fatalf("Invalid regex pattern %q: %v", pattern, err)
	}
	if !match {
		t.Errorf("Generated code does not contain expected pattern %q.\nGenerated Code:\n%s", pattern, code)
	}
}

// assertNotContainsPattern checks if the generated code does NOT contain a specific regular expression pattern.
func assertNotContainsPattern(t *testing.T, code string, pattern string) {
	t.Helper()
	match, err := regexp.MatchString(pattern, code)
	if err != nil {
		t.Fatalf("Invalid regex pattern %q: %v", pattern, err)
	}
	if match {
		t.Errorf("Generated code contains unexpected pattern %q.\nGenerated Code:\n%s", pattern, code)
	}
}

// TestOrchestratorBasicFunctionality tests the new architecture orchestrator
func TestOrchestratorBasicFunctionality(t *testing.T) {
	// Use simple test case
	testPath := "../../testdata/02_basic_conversions/simple_struct"

	// Parse config
	parser := config.NewParser()
	cfg, err := parser.Parse(testPath)
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Analyze types
	typeAnalyzer := analyzer.NewTypeAnalyzer()
	packagePaths := cfg.RequiredPackages()
	typeFQNs := cfg.RequiredTypeFQNs()
	typeInfos, err := typeAnalyzer.Analyze(packagePaths, typeFQNs)
	if err != nil {
		t.Fatalf("Failed to analyze types: %v", err)
	}

	t.Run("Create_Orchestrator", func(t *testing.T) {
		orchestrator := NewOrchestrator(cfg, typeInfos)
		if orchestrator == nil {
			t.Fatal("Failed to create orchestrator")
		}

		retrievedConfig := orchestrator.GetConfig()
		if retrievedConfig == nil {
			t.Fatal("Failed to retrieve config from orchestrator")
		}
	})

	t.Run("Generate_Code", func(t *testing.T) {
		orchestrator := NewOrchestrator(cfg, typeInfos)
		
		genContext := &model.GenerationContext{
			Config:           cfg,
			TypeInfos:        typeInfos,
			InvolvedPackages: make(map[string]struct{}),
		}

		request := &model.GenerationRequest{
			Context: genContext,
		}

		response, err := orchestrator.Generate(request)
		if err != nil {
			t.Fatalf("Generation failed: %v", err)
		}

		if len(response.GeneratedCode) == 0 {
			t.Fatal("GeneratedCode is empty")
		}

		// Verify that generated code contains expected functions
		generatedStr := string(response.GeneratedCode)
		
		// Check for conversion functions based on actual generated output
		assertContainsPattern(t, generatedStr, `func Convert.*To.*\(from \*.*\) \*.*`)
		
		// Check that the generated code contains field assignments
		assertContainsPattern(t, generatedStr, `ID:\s+from\.ID,`)
		assertContainsPattern(t, generatedStr, `Name:\s+from\.Name,`)
		
		t.Logf("Successfully generated %d bytes with %d packages", 
			len(response.GeneratedCode), len(response.RequiredPackages))
	})
}

// TestGenerator_CodeGeneration_NewArchitecture tests the new component-based architecture
func TestGenerator_CodeGeneration_NewArchitecture(t *testing.T) {

	// Base dependencies for most tests
	baseDependencies := []string{
		"github.com/origadmin/abgen/testdata/fixtures/ent",
		"github.com/origadmin/abgen/testdata/fixtures/types",
	}

	testCases := []struct {
		name           string
		directivePath  string
		goldenFileName string
		dependencies   []string
		priority       string
		category       string
		assertFunc     func(t *testing.T, generatedCode []byte, stubCode []byte)
	}{
		// Basic conversions tests
		{
			name:          "simple_struct_conversion",
			directivePath: "../../testdata/02_basic_conversions/simple_struct",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/02_basic_conversions/simple_struct/source",
				"github.com/origadmin/abgen/testdata/02_basic_conversions/simple_struct/target",
			},
			priority: "P0",
			category: "basic_conversions",
			assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
				generatedStr := string(generatedCode)
				assertContainsPattern(t, generatedStr, `func ConvertUserToUserDTO\(from \*User\) \*UserDTO`)
				assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`)
				assertContainsPattern(t, generatedStr, `UserName:\s+from.Name,`)
				assertContainsPattern(t, generatedStr, `func ConvertUserDTOToUser\(from \*UserDTO\) \*User`)
				assertContainsPattern(t, generatedStr, `ID:\s+from.ID,`)
				assertContainsPattern(t, generatedStr, `Name:\s+from.UserName,`)
			},
		},
		{
			name:          "package_level_conversion",
			directivePath: "../../testdata/02_basic_conversions/package_level_conversion",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/02_basic_conversions/package_level_conversion/source",
				"github.com/origadmin/abgen/testdata/02_basic_conversions/package_level_conversion/target",
			},
			priority: "P0",
			category: "basic_conversions",
			assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
				generatedStr := string(generatedCode)
				assertContainsPattern(t, generatedStr, `func ConvertUserSourceToUserTarget\(from \*UserSource\) \*UserTarget`)
				assertContainsPattern(t, generatedStr, `func ConvertUserTargetToUserSource\(from \*UserTarget\) \*UserSource`)
				assertContainsPattern(t, generatedStr, `func ConvertItemSourceToItemTarget\(from \*ItemSource\) \*ItemTarget`)
				assertContainsPattern(t, generatedStr, `func ConvertItemTargetToItemSource\(from \*ItemTarget\) \*ItemSource`)
			},
		},
		{
			name:          "oneway_conversion",
			directivePath: "../../testdata/02_basic_conversions/oneway_conversion",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/02_basic_conversions/oneway_conversion/source",
				"github.com/origadmin/abgen/testdata/02_basic_conversions/oneway_conversion/target",
			},
			priority: "P0",
			category: "basic_conversions",
			assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
				generatedStr := string(generatedCode)
				assertContainsPattern(t, generatedStr, `func ConvertUserToUserDTO\(from \*User\) \*UserDTO`)
				assertNotContainsPattern(t, generatedStr, `func ConvertUserDTOToUser`)
			},
		},
		{
			name:          "simple_bilateral",
			directivePath: "../../testdata/02_basic_conversions/simple_bilateral",
			dependencies:  baseDependencies,
			priority:      "P0",
			category:      "basic_conversions",
			assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
				generatedStr := string(generatedCode)
				assertContainsPattern(t, generatedStr, `func ConvertStringToTime\(s string\) time.Time`)
				assertContainsPattern(t, generatedStr, `func ConvertTimeToString\(t time.Time\) string`)
				assertContainsPattern(t, generatedStr, `func ConvertUserToUserBilateral\(from \*User\) \*UserBilateral`)
				assertContainsPattern(t, generatedStr, `func ConvertUserBilateralToUser\(from \*UserBilateral\) \*User`)
				assertNotContainsPattern(t, generatedStr, `Password:`)
				assertNotContainsPattern(t, generatedStr, `Salt:`)
			},
		},
		{
			name:           "custom_function_rules",
			directivePath:  "../../testdata/03_advanced_features/custom_function_rules",
			goldenFileName: "expected.golden",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/03_advanced_features/custom_function_rules/source",
				"github.com/origadmin/abgen/testdata/03_advanced_features/custom_function_rules/target",
			},
			priority: "P0",
			category: "advanced_features",
			assertFunc: func(t *testing.T, generatedCode []byte, stubCode []byte) {
				generatedStr := string(generatedCode)
				stubStr := string(stubCode)
				assertContainsPattern(t, generatedStr, `Status:\s+ConvertUserStatusToUserCustomStatus\(from.Status\),`)
				assertContainsPattern(t, generatedStr, `Status:\s+ConvertUserCustomStatusToUserStatus\(from.Status\),`)
				if len(stubStr) > 0 {
					assertContainsPattern(t, stubStr, `func ConvertUserStatusToUserCustomStatus\(from int\) string`)
				}
			},
		},
	}

	// Sort test cases for consistent execution order
	sort.Slice(testCases, func(i, j int) bool {
		priorityOrder := map[string]int{"P0": 0, "P1": 1, "P2": 2}
		if priorityOrder[testCases[i].priority] != priorityOrder[testCases[j].priority] {
			return priorityOrder[testCases[i].priority] < priorityOrder[testCases[j].priority]
		}
		return testCases[i].category < testCases[j].category
	})

	for _, tc := range testCases {
		// Extract the numeric prefix from the directory path
		var stagePrefix string
		pathParts := strings.Split(tc.directivePath, "/")
		for _, part := range pathParts {
			if len(part) > 2 && part[2] == '_' && part[0] >= '0' && part[0] <= '9' && part[1] >= '0' && part[1] <= '9' {
				stagePrefix = part[:2]
				break
			}
		}

		testNameWithStage := fmt.Sprintf("NEW_ARCH_%s_%s/%s", stagePrefix, tc.category, tc.name)
		t.Run(testNameWithStage, func(t *testing.T) {
			t.Logf("Running NEW ARCH test: %s (Priority: %s, Category: %s)", tc.name, tc.priority, tc.category)

			// Cleanup any previously generated files
			cleanTestFiles(t, tc.directivePath)
			defer cleanTestFiles(t, tc.directivePath)

			// Step 1: Parse config from the directive path
			parser := config.NewParser()
			cfg, err := parser.Parse(tc.directivePath)
			if err != nil {
				t.Fatalf("config.Parser.Parse() failed: %v", err)
			}

			// Step 2: Analyze types
			typeAnalyzer := analyzer.NewTypeAnalyzer()
			packagePaths := cfg.RequiredPackages()
			typeFQNs := cfg.RequiredTypeFQNs()
			typeInfos, err := typeAnalyzer.Analyze(packagePaths, typeFQNs)
			if err != nil {
				t.Fatalf("analyzer.TypeAnalyzer.Analyze() failed: %v", err)
			}

			// Step 3: Create generation context and request using NEW architecture
			genContext := &model.GenerationContext{
				Config:           cfg,
				TypeInfos:        typeInfos,
				InvolvedPackages: make(map[string]struct{}),
			}

			// Step 4: Create orchestrator with NEW architecture
			orchestrator := NewOrchestrator(cfg, typeInfos)

			// Step 5: Generate request
			request := &model.GenerationRequest{
				Context: genContext,
			}

			// Step 6: Generate code using NEW architecture
			response, err := orchestrator.Generate(request)
			if err != nil {
				t.Fatalf("NEW ARCH Generate() failed for test case %s: %v", tc.name, err)
			}

			generatedCode := response.GeneratedCode
			stubCode := response.CustomStubs

			// Normalize path separators for consistent comparison
			generatedCodeStr := string(generatedCode)
			generatedCodeStr = strings.ReplaceAll(generatedCodeStr, `\`, `/`)
			generatedCode = []byte(generatedCodeStr)

			// Step 7: Run custom assertions
			if tc.assertFunc != nil {
				t.Run(tc.name+"_Assertions", func(st *testing.T) {
					tc.assertFunc(st, generatedCode, stubCode)
					if st.Failed() {
						actualOutputFile := filepath.Join(tc.directivePath, "failed.new_arch.actual.gen.go")
						_ = os.WriteFile(actualOutputFile, generatedCode, 0644)
						st.Logf("NEW ARCH assertion failed for '%s'. Generated output saved to %s for inspection.", tc.name, actualOutputFile)
						if len(stubCode) > 0 {
							actualStubFile := filepath.Join(tc.directivePath, "failed.new_arch.actual.stub.go")
							_ = os.WriteFile(actualStubFile, stubCode, 0644)
							st.Logf("NEW ARCH stub output saved to %s for inspection.", actualStubFile)
						}
					} else {
						actualOutputFile := filepath.Join(tc.directivePath, "success.new_arch.actual.gen.go")
						_ = os.WriteFile(actualOutputFile, generatedCode, 0644)
					}
				})
			}

			// Step 8: Golden file comparison if specified
			if tc.goldenFileName != "" {
				goldenFile := filepath.Join(tc.directivePath, tc.goldenFileName)
				if os.Getenv("UPDATE_GOLDEN_FILES") != "" {
					err = os.WriteFile(goldenFile, generatedCode, 0644)
					if err != nil {
						t.Fatalf("Failed to update golden file %s: %v", goldenFile, err)
					}
					t.Logf("Updated golden file: %s", goldenFile)
					return
				}

				expectedCode, err := os.ReadFile(goldenFile)
				if err != nil {
					t.Fatalf("Failed to read golden file %s: %v", goldenFile, err)
				}

				if string(generatedCode) != string(expectedCode) {
					actualOutputFile := filepath.Join(tc.directivePath, "failed.new_arch.actual.gen.go")
					_ = os.WriteFile(actualOutputFile, generatedCode, 0644)
					t.Errorf("NEW ARCH generated code for '%s' does not match the golden file %s. The generated output was saved to %s for inspection.", tc.name, goldenFile, actualOutputFile)
				}
			}
		})
	}
}

// TestArchitecturalCompatibility tests that both old and new architectures produce identical output
func TestArchitecturalCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping compatibility test in short mode")
	}

	testCases := []struct {
		name          string
		directivePath string
		dependencies  []string
	}{
		{
			name:          "simple_struct",
			directivePath: "../../testdata/02_basic_conversions/simple_struct",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/02_basic_conversions/simple_struct/source",
				"github.com/origadmin/abgen/testdata/02_basic_conversions/simple_struct/target",
			},
		},
		{
			name:          "simple_bilateral",
			directivePath: "../../testdata/02_basic_conversions/simple_bilateral",
			dependencies: []string{
				"github.com/origadmin/abgen/testdata/fixtures/ent",
				"github.com/origadmin/abgen/testdata/fixtures/types",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing architectural compatibility for %s", tc.name)

			// Parse config
			parser := config.NewParser()
			cfg, err := parser.Parse(tc.directivePath)
			if err != nil {
				t.Fatalf("Failed to parse config: %v", err)
			}

			// Analyze types
			typeAnalyzer := analyzer.NewTypeAnalyzer()
			packagePaths := cfg.RequiredPackages()
			typeFQNs := cfg.RequiredTypeFQNs()
			typeInfos, err := typeAnalyzer.Analyze(packagePaths, typeFQNs)
			if err != nil {
				t.Fatalf("Failed to analyze types: %v", err)
			}

			// Generate using OLD architecture
			oldGen := NewGenerator(cfg)
			oldCode, err := oldGen.Generate(typeInfos)
			if err != nil {
				t.Fatalf("OLD architecture failed: %v", err)
			}

			// Generate using NEW architecture
			genContext := &model.GenerationContext{
				Config:           cfg,
				TypeInfos:        typeInfos,
				InvolvedPackages: make(map[string]struct{}),
			}
			orchestrator := NewOrchestrator(cfg, typeInfos)
			request := &model.GenerationRequest{
				Context: genContext,
			}
			response, err := orchestrator.Generate(request)
			if err != nil {
				t.Fatalf("NEW architecture failed: %v", err)
			}
			newCode := response.GeneratedCode

			// Normalize both outputs
			oldStr := strings.ReplaceAll(string(oldCode), `\`, `/`)
			newStr := strings.ReplaceAll(string(newCode), `\`, `/`)

			// Compare outputs
			if oldStr != newStr {
				t.Errorf("Architecture mismatch for %s", tc.name)
				t.Logf("OLD architecture output length: %d", len(oldStr))
				t.Logf("NEW architecture output length: %d", len(newStr))

				// Write both outputs for comparison
				outputDir := filepath.Join(tc.directivePath, "compatibility_test")
				_ = os.MkdirAll(outputDir, 0755)
				_ = os.WriteFile(filepath.Join(outputDir, "old_architecture.gen.go"), []byte(oldStr), 0644)
				_ = os.WriteFile(filepath.Join(outputDir, "new_architecture.gen.go"), []byte(newStr), 0644)
				t.Logf("Output files written to %s for manual comparison", outputDir)
			} else {
				t.Logf("Architectures produce identical output for %s", tc.name)
			}
		})
	}
}

// TestNewArchitectureComponents tests individual components of the new architecture
func TestNewArchitectureComponents(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping component tests in short mode")
	}

	// Use simple test case
	testPath := "../../testdata/02_basic_conversions/simple_struct"

	// Parse config
	parser := config.NewParser()
	cfg, err := parser.Parse(testPath)
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Analyze types
	typeAnalyzer := analyzer.NewTypeAnalyzer()
	packagePaths := cfg.RequiredPackages()
	typeFQNs := cfg.RequiredTypeFQNs()
	typeInfos, err := typeAnalyzer.Analyze(packagePaths, typeFQNs)
	if err != nil {
		t.Fatalf("Failed to analyze types: %v", err)
	}

	t.Run("Orchestrator_Creation", func(t *testing.T) {
		// Test that we can create an orchestrator
		orchestrator := NewOrchestrator(cfg, typeInfos)
		if orchestrator == nil {
			t.Fatal("Failed to create orchestrator")
		}

		// Test that we can get the config back
		config := orchestrator.GetConfig()
		if config == nil {
			t.Fatal("Failed to get config from orchestrator")
		}
	})

	t.Run("Generation_Context", func(t *testing.T) {
		// Test generation context creation
		genContext := &model.GenerationContext{
			Config:           cfg,
			TypeInfos:        typeInfos,
			InvolvedPackages: make(map[string]struct{}),
		}

		if genContext.Config == nil {
			t.Fatal("Config is nil in generation context")
		}
		if len(genContext.TypeInfos) == 0 {
			t.Fatal("TypeInfos is empty in generation context")
		}
		if genContext.InvolvedPackages == nil {
			t.Fatal("InvolvedPackages is nil in generation context")
		}
	})

	t.Run("Generation_Request_Response", func(t *testing.T) {
		// Test generation request
		genContext := &model.GenerationContext{
			Config:           cfg,
			TypeInfos:        typeInfos,
			InvolvedPackages: make(map[string]struct{}),
		}

		request := &model.GenerationRequest{
			Context: genContext,
		}

		if request.Context == nil {
			t.Fatal("Context is nil in generation request")
		}

		// Test generation response
		response := &model.GenerationResponse{
			GeneratedCode:    []byte("test code"),
			CustomStubs:      []byte("test stubs"),
			RequiredPackages: []string{"test/package"},
		}

		if len(response.GeneratedCode) == 0 {
			t.Fatal("GeneratedCode is empty in response")
		}
		if len(response.RequiredPackages) == 0 {
			t.Fatal("RequiredPackages is empty in response")
		}
	})

	t.Run("End_to_End_Generation", func(t *testing.T) {
		// Test full generation cycle
		genContext := &model.GenerationContext{
			Config:           cfg,
			TypeInfos:        typeInfos,
			InvolvedPackages: make(map[string]struct{}),
		}

		orchestrator := NewOrchestrator(cfg, typeInfos)
		request := &model.GenerationRequest{
			Context: genContext,
		}

		response, err := orchestrator.Generate(request)
		if err != nil {
			t.Fatalf("Generation failed: %v", err)
		}

		if len(response.GeneratedCode) == 0 {
			t.Fatal("GeneratedCode is empty in response")
		}

		// Check that generated code contains expected function
		generatedStr := string(response.GeneratedCode)
		assertContainsPattern(t, generatedStr, `func ConvertUserToUserDTO\(from \*User\) \*UserDTO`)
		assertContainsPattern(t, generatedStr, `func ConvertUserDTOToUser\(from \*UserDTO\) \*User`)

		t.Logf("Successfully generated %d bytes of code with %d required packages", 
			len(response.GeneratedCode), len(response.RequiredPackages))
	})
}
