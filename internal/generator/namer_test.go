package generator

import (
	"testing"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

func TestNamer_GetFunctionName(t *testing.T) {
	ruleSet := config.NewRuleSet()
	namer := NewNamer(ruleSet)
	
	sourceType := &model.Type{
		Name: "User",
		Kind: model.TypeKindStruct,
	}
	
	targetType := &model.Type{
		Name: "UserDTO",
		Kind: model.TypeKindStruct,
	}
	
	funcName := namer.GetFunctionName(sourceType, targetType)
	expected := "ConvertUserToUserDTO"
	
	if funcName != expected {
		t.Errorf("Expected function name '%s', got '%s'", expected, funcName)
	}
}

func TestNamer_GetFunctionName_PointerTypes(t *testing.T) {
	ruleSet := config.NewRuleSet()
	namer := NewNamer(ruleSet)
	
	sourceType := &model.Type{
		Name:       "User",
		Kind:       model.TypeKindStruct,
		IsPointer:  true,
	}
	
	targetType := &model.Type{
		Name:      "UserDTO",
		Kind:      model.TypeKindStruct,
		IsPointer: false,
	}
	
	funcName := namer.GetFunctionName(sourceType, targetType)
	expected := "ConvertUserToUserDTO"
	
	if funcName != expected {
		t.Errorf("Expected function name '%s', got '%s'", expected, funcName)
	}
}

func TestNamer_GetFunctionName_SliceTypes(t *testing.T) {
	ruleSet := config.NewRuleSet()
	namer := NewNamer(ruleSet)
	
	sourceType := &model.Type{
		Name: "[]User",
		Kind: model.TypeKindSlice,
		ElementType: &model.Type{
			Name: "User",
			Kind: model.TypeKindStruct,
		},
	}
	
	targetType := &model.Type{
		Name: "[]UserDTO",
		Kind: model.TypeKindSlice,
		ElementType: &model.Type{
			Name: "UserDTO",
			Kind: model.TypeKindStruct,
		},
	}
	
	funcName := namer.GetFunctionName(sourceType, targetType)
	expected := "ConvertUserToUserDTO"
	
	if funcName != expected {
		t.Errorf("Expected function name '%s', got '%s'", expected, funcName)
	}
}