package complex_types

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
