package all_complex_types

import (
	"time"

	"github.com/origadmin/abgen/testdata/01_type_analysis/complex_type_parsing/external"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/00_complex_type_parsing/external,alias=ext
//go:abgen:convert:source:suffix="Source"
//go:abgen:convert:target:suffix="Target"
//go:abgen:pair:packages="ext,all_complex_types"

// --- Basic Types for Parsing ---

type User struct {
	ID        int64
	Name      string
	Email     string
	CreatedAt time.Time
	Status    external.Status
}

type Product struct {
	ProductID string
	Name      string
	Price     float64
}

type UserAlias = external.User

// --- Complex Types for Parsing ---

// BaseStruct is a simple struct used for building more complex types.
type BaseStruct struct {
	ID   int
	Name string
}

// --- Defined Types (type T U) ---

// DefinedPtr is a defined type based on a pointer to a struct.
type DefinedPtr *BaseStruct

// DefinedSliceOfPtrs is a defined type based on a slice of pointers to structs.
type DefinedSliceOfPtrs []*BaseStruct

// DefinedPtrToSlice is a defined type based on a pointer to a slice of structs.
type DefinedPtrToSlice *[]BaseStruct

// DefinedSliceOfSlices is a defined type based on a slice of slices of structs.
type DefinedSliceOfSlices [][]BaseStruct

// DefinedMap is a defined type based on a map.
type DefinedMap map[string]*BaseStruct

// DefinedPtrToSliceOfPtrs is a defined type for a pointer to a slice of pointers.
type DefinedPtrToSliceOfPtrs *[]*BaseStruct

// DefinedPtrToSliceOfSlices is a defined type for a pointer to a slice of slices.
type DefinedPtrToSliceOfSlices *[]*[]BaseStruct

// TriplePtr is a defined type for a triple pointer to a struct.
type TriplePtr ***BaseStruct

// MyPtr is a defined type for a pointer to an int.
type MyPtr *int

// --- Alias Types (type T = U) ---

// AliasPtr is an alias for a pointer to a struct.
type AliasPtr = *BaseStruct

// AliasSliceOfPtrs is an alias for a slice of pointers to structs.
type AliasSliceOfPtrs = []*BaseStruct

// AliasPtrToSlice is an alias for a pointer to a slice of structs.
type AliasPtrToSlice = *[]BaseStruct

// AliasSliceOfSlices is an alias for a slice of slices of structs.
type AliasSliceOfSlices = [][]BaseStruct

// AliasMap is an alias for a map.
type AliasMap = map[string]*BaseStruct

// AliasPtrToSliceOfPtrs is an alias for a pointer to a slice of pointers.
type AliasPtrToSliceOfPtrs = *[]*BaseStruct

// AliasPtrToSliceOfSlices is an alias for a pointer to a slice of slices.
type AliasPtrToSliceOfSlices = *[]*[]BaseStruct
