package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/generator/components"
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
			name:     "Slice of Struct Value Type ([]types.User)",
			info:     sliceOfUser,
			expected: "[]types.User",
		},
		{
			name:     "Slice of Primitive ([]int)",
			info:     sliceOfInt,
			expected: "[]int",
		},
		{
			name:     "Array of Struct Value Type ([5]types.User)",
			info:     array5OfUser,
			expected: "[5]types.User",
		},
		{
			name:     "Map of String to Struct Value Type (map[string]types.User)",
			info:     mapOfStringToUser,
			expected: "map[string]types.User",
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
			name:     "Slice of Named Struct Type ([]ent.Resource)",
			info:     sliceOfEntResource,
			expected: "[]ent.Resource",
		},
		{
			name:     "Slice of Named Type wrapping Struct ([]mypkg.MyResource)",
			info:     sliceOfMyResourceNamedType,
			expected: "[]mypkg.MyResource",
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

// TestNamer_GetTypeString_SliceTypeAliasComprehensive 专门测试切片类型别名的全面覆盖
func TestNamer_GetTypeString_SliceTypeAliasComprehensive(t *testing.T) {
	// Mock TypeInfo objects
	userStructType := &model.TypeInfo{Name: "User", ImportPath: "types", Kind: model.Struct}
	orderStructType := &model.TypeInfo{Name: "Order", ImportPath: "types", Kind: model.Struct}
	intPrimitiveType := &model.TypeInfo{Name: "int", Kind: model.Primitive}

	// 指针类型
	pointerToUser := &model.TypeInfo{Kind: model.Pointer, Underlying: userStructType}
	pointerToOrder := &model.TypeInfo{Kind: model.Pointer, Underlying: orderStructType}

	// 测试各种切片类型组合
	testCases := []struct {
		name     string
		info     *model.TypeInfo
		expected string
	}{
		// 基本值类型切片
		{
			name:     "Slice of Struct Value Type ([]types.User)",
			info:     &model.TypeInfo{Kind: model.Slice, Underlying: userStructType},
			expected: "[]types.User",
		},
		{
			name:     "Slice of Struct Pointer Type ([]*types.User)",
			info:     &model.TypeInfo{Kind: model.Slice, Underlying: pointerToUser},
			expected: "[]*types.User",
		},
		// 混合类型切片
		{
			name:     "Slice of Mixed Struct Types ([]types.Order)",
			info:     &model.TypeInfo{Kind: model.Slice, Underlying: orderStructType},
			expected: "[]types.Order",
		},
		{
			name:     "Slice of Mixed Struct Pointer Types ([]*types.Order)",
			info:     &model.TypeInfo{Kind: model.Slice, Underlying: pointerToOrder},
			expected: "[]*types.Order",
		},
		// 基本类型切片
		{
			name:     "Slice of Primitive ([]int)",
			info:     &model.TypeInfo{Kind: model.Slice, Underlying: intPrimitiveType},
			expected: "[]int",
		},
		// 数组类型
		{
			name:     "Array of Struct Value Type ([10]types.User)",
			info:     &model.TypeInfo{Kind: model.Array, ArrayLen: 10, Underlying: userStructType},
			expected: "[10]types.User",
		},
		{
			name:     "Array of Struct Pointer Type ([10]*types.User)",
			info:     &model.TypeInfo{Kind: model.Array, ArrayLen: 10, Underlying: pointerToUser},
			expected: "[10]*types.User",
		},
		// 映射类型
		{
			name:     "Map of String to Struct Value Type (map[string]types.User)",
			info:     &model.TypeInfo{Kind: model.Map, KeyType: &model.TypeInfo{Name: "string", Kind: model.Primitive}, Underlying: userStructType},
			expected: "map[string]types.User",
		},
		{
			name:     "Map of String to Struct Pointer Type (map[string]*types.User)",
			info:     &model.TypeInfo{Kind: model.Map, KeyType: &model.TypeInfo{Name: "string", Kind: model.Primitive}, Underlying: pointerToUser},
			expected: "map[string]*types.User",
		},
		// 边界情况：空切片
		{
			name:     "Slice of Empty Interface ([]interface{})",
			info:     &model.TypeInfo{Kind: model.Slice, Underlying: &model.TypeInfo{Name: "interface{}", Kind: model.Interface}},
			expected: "[]interface{}",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			namer := NewNamer(&config.Config{}, make(map[string]string))
			result := namer.GetTypeString(tc.info)
			assert.Equal(t, tc.expected, result, "Test case '%s' failed: expected '%s', got '%s'", tc.name, tc.expected, result)
		})
	}
}

// TestNamer_GetTypeString_EdgeCases 测试边界情况
func TestNamer_GetTypeString_EdgeCases(t *testing.T) {
	testCases := []struct {
		name     string
		info     *model.TypeInfo
		expected string
	}{
		{
			name:     "Nil TypeInfo",
			info:     nil,
			expected: "",
		},
		{
			name:     "Nil Underlying in Pointer",
			info:     &model.TypeInfo{Kind: model.Pointer, Underlying: nil},
			expected: "*",
		},
		{
			name:     "Nil Underlying in Slice",
			info:     &model.TypeInfo{Kind: model.Slice, Underlying: nil},
			expected: "[]",
		},
		{
			name:     "Nil Underlying in Array",
			info:     &model.TypeInfo{Kind: model.Array, ArrayLen: 5, Underlying: nil},
			expected: "[5]",
		},
		{
			name:     "Nil KeyType in Map",
			info:     &model.TypeInfo{Kind: model.Map, KeyType: nil, Underlying: &model.TypeInfo{Name: "int", Kind: model.Primitive}},
			expected: "map[interface{}]int",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			namer := NewNamer(&config.Config{}, make(map[string]string))
			result := namer.GetTypeString(tc.info)
			assert.Equal(t, tc.expected, result, "Test case '%s' failed", tc.name)
		})
	}
}

// TestNamer_GetTypeString_PointerSliceAliases 专门测试指针类型切片别名的全面覆盖
func TestNamer_GetTypeString_PointerSliceAliases(t *testing.T) {
	// Mock TypeInfo objects for pointer slice types
	userPPType := &model.TypeInfo{Name: "UserPP", ImportPath: "types", Kind: model.Struct}
	userPV2Type := &model.TypeInfo{Name: "UserPV2", ImportPath: "types", Kind: model.Struct}
	userPPPType := &model.TypeInfo{Name: "UserPPP", ImportPath: "types", Kind: model.Struct}

	// Pointer types
	pointerToUserPP := &model.TypeInfo{Kind: model.Pointer, Underlying: userPPType}
	pointerToUserPV2 := &model.TypeInfo{Kind: model.Pointer, Underlying: userPV2Type}
	pointerToUserPPP := &model.TypeInfo{Kind: model.Pointer, Underlying: userPPPType}

	// Slice of pointer types
	sliceOfPointerToUserPP := &model.TypeInfo{Kind: model.Slice, Underlying: pointerToUserPP}
	sliceOfPointerToUserPV2 := &model.TypeInfo{Kind: model.Slice, Underlying: pointerToUserPV2}
	sliceOfPointerToUserPPP := &model.TypeInfo{Kind: model.Slice, Underlying: pointerToUserPPP}

	// Pointer to slice of pointer types
	pointerToSliceOfPointerToUserPPP := &model.TypeInfo{Kind: model.Pointer, Underlying: sliceOfPointerToUserPPP}

	testCases := []struct {
		name     string
		info     *model.TypeInfo
		expected string
	}{
		{
			name:     "Slice of Pointer to Struct ([]*types.UserPP)",
			info:     sliceOfPointerToUserPP,
			expected: "[]*types.UserPP",
		},
		{
			name:     "Slice of Pointer to Struct ([]*types.UserPV2)",
			info:     sliceOfPointerToUserPV2,
			expected: "[]*types.UserPV2",
		},
		{
			name:     "Slice of Pointer to Struct ([]*types.UserPPP)",
			info:     sliceOfPointerToUserPPP,
			expected: "[]*types.UserPPP",
		},
		{
			name:     "Pointer to Slice of Pointer to Struct (*[]*types.UserPPP)",
			info:     pointerToSliceOfPointerToUserPPP,
			expected: "*[]*types.UserPPP",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			namer := NewNamer(&config.Config{}, make(map[string]string))
			result := namer.GetTypeString(tc.info)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestAliasManager_PointerSliceAliasGeneration 测试指针类型切片别名的生成
func TestAliasManager_PointerSliceAliasGeneration(t *testing.T) {
	// Mock TypeInfo objects for pointer slice types
	userPPType := &model.TypeInfo{Name: "UserPP", ImportPath: "source", Kind: model.Struct}
	userPV2Type := &model.TypeInfo{Name: "UserPV2", ImportPath: "source", Kind: model.Struct}

	// Pointer types
	pointerToUserPP := &model.TypeInfo{Kind: model.Pointer, Underlying: userPPType}
	pointerToUserPV2 := &model.TypeInfo{Kind: model.Pointer, Underlying: userPV2Type}

	// Slice of pointer types
	sliceOfPointerToUserPP := &model.TypeInfo{Kind: model.Slice, Underlying: pointerToUserPP}
	sliceOfPointerToUserPV2 := &model.TypeInfo{Kind: model.Slice, Underlying: pointerToUserPV2}

	// Create mock config with conversion rules and naming rules
	config := &config.Config{
		ConversionRules: []*config.ConversionRule{
			{
				SourceType: "source.ContainerPP",
				TargetType: "target.ContainerPP",
			},
			{
				SourceType: "source.ContainerPV2",
				TargetType: "target.ContainerPV2",
			},
		},
		NamingRules: config.NamingRules{
			SourceSuffix: "Source", // 添加源类型后缀
		},
	}

	// Create type info map - 添加指针类型切片的唯一键
	typeInfos := map[string]*model.TypeInfo{
		"source.ContainerPP": {
			Name: "ContainerPP", ImportPath: "source", Kind: model.Struct,
			Fields: []*model.FieldInfo{
				{Name: "Users", Type: sliceOfPointerToUserPP},
			},
		},
		"target.ContainerPP": {
			Name: "ContainerPP", ImportPath: "target", Kind: model.Struct,
			Fields: []*model.FieldInfo{
				{Name: "Users", Type: sliceOfPointerToUserPP},
			},
		},
		"source.ContainerPV2": {
			Name: "ContainerPV2", ImportPath: "source", Kind: model.Struct,
			Fields: []*model.FieldInfo{
				{Name: "Users", Type: sliceOfPointerToUserPV2},
			},
		},
		"target.ContainerPV2": {
			Name: "ContainerPV2", ImportPath: "target", Kind: model.Struct,
			Fields: []*model.FieldInfo{
				{Name: "Users", Type: sliceOfPointerToUserPV2},
			},
		},
		// 添加指针类型切片到typeInfos映射中
		sliceOfPointerToUserPP.UniqueKey():  sliceOfPointerToUserPP,
		sliceOfPointerToUserPV2.UniqueKey(): sliceOfPointerToUserPV2,
	}

	// Create alias manager
	importManager := &mockImportManager{}
	nameGenerator := components.NewNameGenerator(config, importManager)

	aliasManager := components.NewAliasManager(config, importManager, nameGenerator, typeInfos)

	// Populate aliases
	aliasManager.PopulateAliases()

	// Get aliases to render
	aliases := aliasManager.GetAliasesToRender()

	// Verify that pointer slice aliases are generated
	foundUserPPsSource := false
	foundUserPV2sSource := false

	for _, alias := range aliases {
		if alias.AliasName == "UserPPsSource" {
			foundUserPPsSource = true
			assert.Equal(t, "[]*source.UserPP", alias.OriginalTypeName)
		}
		if alias.AliasName == "UserPV2sSource" {
			foundUserPV2sSource = true
			assert.Equal(t, "[]*source.UserPV2", alias.OriginalTypeName)
		}
	}

	assert.True(t, foundUserPPsSource, "UserPPsSource alias should be generated")
	assert.True(t, foundUserPV2sSource, "UserPV2sSource alias should be generated")
}

// Mock import manager for testing
type mockImportManager struct{}

func (m *mockImportManager) AddAs(pkgPath, alias string) string {
	return ""
}

func (m *mockImportManager) GetAllImports() map[string]string {
	return map[string]string{}
}

func (m *mockImportManager) Add(importPath string) string {
	return ""
}

func (m *mockImportManager) GetAlias(pkgPath string) (string, bool) {
	return "", false
}
