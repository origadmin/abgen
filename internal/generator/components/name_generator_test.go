package components

import (
	"testing"

	"github.com/origadmin/abgen/internal/model"
)

// mockAliasManager is a mock implementation of the model.AliasManager for testing.
type mockAliasManager struct {
	aliasMap map[string]string
}

func (m *mockAliasManager) LookupAlias(uniqueKey string) (string, bool) {
	alias, ok := m.aliasMap[uniqueKey]
	return alias, ok
}
func (m *mockAliasManager) PopulateAliases()                  {}
func (m *mockAliasManager) GetAllAliases() map[string]string    { return m.aliasMap }
func (m *mockAliasManager) GetAliasedTypes() map[string]*model.TypeInfo { return nil }
func (m *mockAliasManager) GetAlias(info *model.TypeInfo) (string, bool) {
	return m.LookupAlias(info.UniqueKey())
}
func (m *mockAliasManager) IsUserDefined(uniqueKey string) bool { return false }
func (m *mockAliasManager) GetSourcePath() string               { return "" }
func (m *mockAliasManager) GetTargetPath() string               { return "" }

func TestNameGenerator_ConversionFunctionName(t *testing.T) {
	// Primitives
	intType := newPrimitive("int")
	stringType := newPrimitive("string")

	// Structs
	userStruct := newStruct("User", "a/b", nil)
	userDTOStruct := newStruct("UserDTO", "c/d", nil)

	// Slices of structs
	userSlice := newSlice(userStruct)
	userDTOSlice := newSlice(userDTOStruct)

	// Slices of pointers to structs
	pointerUserSlice := newSlice(newPointer(userStruct))
	pointerUserDTOSlice := newSlice(newPointer(userDTOStruct))

	// Mock alias manager with correct pluralization provided by AliasManager
	mockAM := &mockAliasManager{
		aliasMap: map[string]string{
			"a/b.User":          "UserSource",
			"c/d.UserDTO":       "UserTarget",
			"[]a/b.User":        "UsersSource", // Correct pluralization
			"[]c/d.UserDTO":     "UsersTarget", // Correct pluralization
			"[]*a/b.User":       "UsersSource", // Slices of pointers should have the same alias
			"[]*c/d.UserDTO":    "UsersTarget",
		},
	}

	ng := NewNameGenerator(mockAM)

	testCases := []struct {
		name     string
		source   *model.TypeInfo
		target   *model.TypeInfo
		expected string
	}{
		{"Primitives", intType, stringType, "ConvertIntToString"},
		{"Structs with Alias", userStruct, userDTOStruct, "ConvertUserSourceToUserTarget"},
		{"Slices of Structs with Alias", userSlice, userDTOSlice, "ConvertUsersSourceToUsersTarget"},
		{"Slices of Pointers with Alias", pointerUserSlice, pointerUserDTOSlice, "ConvertUsersSourceToUsersTarget"},
		{"Pointer to Struct", newPointer(userStruct), userDTOStruct, "ConvertUserSourceToUserTarget"},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			got := ng.ConversionFunctionName(tt.source, tt.target)
			if got != tt.expected {
				t.Errorf("ConversionFunctionName() = %v, want %v", got, tt.expected)
			}
		})
	}
}
