package array_slice_test

import (
	_ "github.com/origadmin/abgen/testdata/06_regression/array_slice_fix/array_slice_test/source"
	_ "github.com/origadmin/abgen/testdata/06_regression/array_slice_fix/array_slice_test/target"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/06_regression/array_slice_fix/array_slice_test/source,alias=source
//go:abgen:package:path=github.com/origadmin/abgen/testdata/06_regression/array_slice_fix/array_slice_test/target,alias=target

// Test case for array/slice conversion issues
// Tests: direct slice-to-slice conversion for array aliases

//go:abgen:pair:packages="source,target"
//go:abgen:convert="source.Department,target.Department"
//go:abgen:convert:direction="both"
//go:abgen:convert:target:suffix="PB"

// Expected: Conversion functions for slice types without extra *
// Should generate: ConvertDepartmentsToDepartmentsPB(from Departments) DepartmentsPB
// Not: ConvertDepartmentPBToDepartments(from *DepartmentPB) *Departments
