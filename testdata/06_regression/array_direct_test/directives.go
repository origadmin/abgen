package array_direct_test

import (
	_ "github.com/origadmin/abgen/testdata/03_advanced_features/slice_conversion_test_case/source"
	_ "github.com/origadmin/abgen/testdata/03_advanced_features/slice_conversion_test_case/target"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/06_regression/array_direct_test/source,alias=source
//go:abgen:package:path=github.com/origadmin/abgen/testdata/06_regression/array_direct_test/target,alias=target

// Test for direct slice conversion - no extra pointers or suffixes

//go:abgen:pair:packages="source,target"
//go:abgen:convert="source.Department,target.Department"
//go:abgen:convert:direction="both"

// Expected: Generate slice conversion functions like:
// ConvertDepartmentsToDepartmentsPB(from Departments) DepartmentsPB
// ConvertDepartmentsPBToDepartments(from DepartmentsPB) Departments
