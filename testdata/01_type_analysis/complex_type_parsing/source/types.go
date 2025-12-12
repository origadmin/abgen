//go:build abgen
// +build abgen

package all_complex_types

import (
	"time"

	"github.com/origadmin/abgen/testdata/01_type_analysis/01_complex_types/external"
)

// BaseStruct is a simple struct used as an element in more complex types.
type BaseStruct struct {
	ID   int
	Name string
}

// User demonstrates a struct with fields of various complex types.
type User struct {
	PrimaryID      int64
	Alias          UserAlias
	SliceOfPtrs    []*BaseStruct
	PtrToSlice     *[]BaseStruct
	ComplexMap     map[string]*[]*BaseStruct
	unexportedField string
}

// UserAlias is an alias to a struct from an external package.
type UserAlias = external.User

// DefinedPtr is a defined type for a pointer to a struct.
type DefinedPtr *BaseStruct

// AliasPtr is an alias for a pointer to a struct.
type AliasPtr = *BaseStruct

// DefinedSliceOfPtrs is a defined type for a slice of pointers.
type DefinedSliceOfPtrs []*BaseStruct

// AliasSliceOfPtrs is an alias for a slice of pointers.
type AliasSliceOfPtrs = []*BaseStruct

// DefinedPtrToSlice is a defined type for a pointer to a slice.
type DefinedPtrToSlice *[]BaseStruct

// AliasPtrToSlice is an alias for a pointer to a slice.
type AliasPtrToSlice = *[]BaseStruct

// DefinedSliceOfSlices is a defined type for a slice of slices.
type DefinedSliceOfSlices [][]BaseStruct

// AliasSliceOfSlices is an alias for a slice of slices.
type AliasSliceOfSlices = [][]BaseStruct

// DefinedMap is a defined type for a map.
type DefinedMap map[string]*BaseStruct

// AliasMap is an alias for a map.
type AliasMap = map[string]*BaseStruct

// TriplePtr is a defined type for a triple pointer.
type TriplePtr ***BaseStruct

// MyPtr is a defined type for a pointer to a primitive.
type MyPtr *int

// EmbeddedStruct demonstrates struct embedding.
type EmbeddedStruct struct {
	BaseStruct
	Timestamp time.Time
}
