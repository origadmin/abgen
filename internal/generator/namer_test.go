package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

func TestNamer_GetAlias_FinalCorrectLogic(t *testing.T) {
	// --- Base Types ---
	userType := &model.TypeInfo{Name: "User", Kind: model.Struct}
	// A singular struct type whose name is already in plural form.
	usersStructType := &model.TypeInfo{Name: "Users", Kind: model.Struct}

	// --- Container Types ---
	sliceOfUserType := &model.TypeInfo{Kind: model.Slice, Underlying: userType}
	mapOfUserType := &model.TypeInfo{Kind: model.Map, Underlying: userType}
	// A slice of the singular struct that has a plural name.
	sliceOfUsersStructType := &model.TypeInfo{Kind: model.Slice, Underlying: usersStructType}
	// A named slice type (e.g., `type UserList []User`)
	userListType := &model.TypeInfo{Name: "UserList", Kind: model.Slice, Underlying: userType}

	// --- Configs ---
	fullRulesConfig := &config.Config{
		NamingRules: config.NamingRules{
			SourcePrefix: "Src",
			SourceSuffix: "Model",
			TargetPrefix: "Dest",
			TargetSuffix: "DTO",
		},
	}

	// --- Test Cases ---
	testCases := []struct {
		name          string
		cfg           *config.Config
		info          *model.TypeInfo
		isSource      bool
		expectedAlias string
	}{
		// --- Scenario: Full rules applied to various types ---
		{name: "Struct (Source)", cfg: fullRulesConfig, info: userType, isSource: true, expectedAlias: "SrcUserModel"},
		{name: "Slice (Source)", cfg: fullRulesConfig, info: sliceOfUserType, isSource: true, expectedAlias: "SrcUsersModel"},
		{name: "Map (Source)", cfg: fullRulesConfig, info: mapOfUserType, isSource: true, expectedAlias: "SrcUserMapModel"},
		{name: "Struct (Target)", cfg: fullRulesConfig, info: userType, isSource: false, expectedAlias: "DestUserDTO"},
		{name: "Slice (Target)", cfg: fullRulesConfig, info: sliceOfUserType, isSource: false, expectedAlias: "DestUsersDTO"},
		{name: "Map (Target)", cfg: fullRulesConfig, info: mapOfUserType, isSource: false, expectedAlias: "DestUserMapDTO"},

		// --- Scenario: Suffix is always appended, no stripping ---
		{
			name:          "Suffix Appended Even if Base Name Contains It",
			cfg:           fullRulesConfig,
			info:          &model.TypeInfo{Name: "UserDTO", Kind: model.Struct},
			isSource:      false,
			expectedAlias: "DestUserDTODTO", // Correctly appends, no stripping
		},

		// --- Scenario: Unconditional 's' for Slice/Array types ---
		{
			name:          "Struct Named 'Users' (singular)",
			cfg:           fullRulesConfig,
			info:          usersStructType, // info is a single struct named "Users"
			isSource:      true,
			expectedAlias: "SrcUsersModel", // No 's' added as it's not a slice/array
		},
		{
			name:          "Slice of Struct Named 'Users' (unconditional 's')",
			cfg:           fullRulesConfig,
			info:          sliceOfUsersStructType, // info is a slice of "Users" structs
			isSource:      true,
			expectedAlias: "SrcUserssModel", // Now correctly becomes "Userss" to ensure uniqueness
		},

		// --- Scenario: Named Slice Type ---
		{
			name:          "Named Slice Type (UserList) (unconditional 's')",
			cfg:           fullRulesConfig,
			info:          userListType, // info is a named slice type "UserList"
			isSource:      false,
			expectedAlias: "DestUserListsDTO", // "UserList" + "s" + "DTO"
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			namer := NewNamer(tc.cfg, make(map[string]string))
			alias := namer.GetAlias(tc.info, tc.isSource)
			assert.Equal(t, tc.expectedAlias, alias)
		})
	}
}

func TestNamer_GetTypeString(t *testing.T) {
	// Mock TypeInfo objects
	userStructType := &model.TypeInfo{Name: "User", ImportPath: "types", Kind: model.Struct}
	intPrimitiveType := &model.TypeInfo{Name: "int", Kind: model.Primitive}
	stringPrimitiveType := &model.TypeInfo{Name: "string", Kind: model.Primitive}

	// Complex types
	pointerToUser := &model.TypeInfo{Kind: model.Pointer, Underlying: userStructType}
	sliceOfUser := &model.TypeInfo{Kind: model.Slice, Underlying: userStructType}
	sliceOfInt := &model.TypeInfo{Kind: model.Slice, Underlying: intPrimitiveType}

	// Array with specific length
	array5OfUser := &model.TypeInfo{Kind: model.Array, ArrayLen: 5, Underlying: userStructType}

	// Map with specific key type
	mapOfStringToUser := &model.TypeInfo{Kind: model.Map, KeyType: stringPrimitiveType, Underlying: userStructType}

	// Named struct type (e.g., ent.Resource)
	entResourceStruct := &model.TypeInfo{Name: "Resource", ImportPath: "ent", Kind: model.Struct}
	// Named type wrapping a struct (e.g., type MyResource ent.Resource)
	myResourceNamedType := &model.TypeInfo{Name: "MyResource", ImportPath: "mypkg", Kind: model.Named, Underlying: entResourceStruct}
	// Slice of a named type that ultimately points to a struct (e.g., []ent.Resource)
	sliceOfEntResource := &model.TypeInfo{Kind: model.Slice, Underlying: entResourceStruct}
	// Slice of a named type that ultimately points to a struct (e.g., []mypkg.MyResource)
	sliceOfMyResourceNamedType := &model.TypeInfo{Kind: model.Slice, Underlying: myResourceNamedType}

	testCases := []struct {
		name     string
		info     *model.TypeInfo
		expected string
	}{
		{
			name:     "Primitive Type (int)",
			info:     intPrimitiveType,
			expected: "int",
		},
		{
			name:     "Primitive Type (string)",
			info:     stringPrimitiveType,
			expected: "string",
		},
		{
			name:     "Struct Type (types.User)",
			info:     userStructType,
			expected: "types.User",
		},
		{
			name:     "Pointer to Struct (*types.User)",
			info:     pointerToUser,
			expected: "*types.User",
		},
		{
			name:     "Slice of Struct ([]*types.User)",
			info:     sliceOfUser,
			expected: "[]*types.User",
		},
		{
			name:     "Slice of Primitive ([]int)",
			info:     sliceOfInt,
			expected: "[]int",
		},
		{
			name:     "Array of Struct ([5]*types.User)",
			info:     array5OfUser,
			expected: "[5]*types.User",
		},
		{
			name:     "Map of String to Struct (map[string]*types.User)",
			info:     mapOfStringToUser,
			expected: "map[string]*types.User",
		},
		{
			name:     "Named Struct Type (ent.Resource)",
			info:     entResourceStruct,
			expected: "ent.Resource",
		},
		{
			name:     "Named Type wrapping Struct (mypkg.MyResource)",
			info:     myResourceNamedType,
			expected: "mypkg.MyResource",
		},
		{
			name:     "Slice of Named Struct Type ([]*ent.Resource)",
			info:     sliceOfEntResource,
			expected: "[]*ent.Resource",
		},
		{
			name:     "Slice of Named Type wrapping Struct ([]*mypkg.MyResource)",
			info:     sliceOfMyResourceNamedType,
			expected: "[]*mypkg.MyResource",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			namer := NewNamer(&config.Config{}, make(map[string]string)) // Config not relevant for GetTypeString
			result := namer.GetTypeString(tc.info)
			assert.Equal(t, tc.expected, result)
		})
	}
}
