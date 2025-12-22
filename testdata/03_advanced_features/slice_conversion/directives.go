package directives

import (
	"github.com/origadmin/abgen/testdata/03_advanced_features/slice_conversion/source"
	"github.com/origadmin/abgen/testdata/03_advanced_features/slice_conversion/target"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/03_advanced_features/slice_conversion/source,alias=source
//go:abgen:package:path=github.com/origadmin/abgen/testdata/03_advanced_features/slice_conversion/target,alias=target

// Phase for Slice Conversions
// Tests: struct fields containing slices of other convertible types.

//go:abgen:pair:packages="source,target"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="PB"

// Expected: Conversion functions for structs with slice fields.
type (
	DepartmentEdges   = source.DepartmentEdges
	DepartmentEdgesPB = target.DepartmentEdges
)
