package directives

import (
	_ "github.com/origadmin/abgen/testdata/06_regression/map_string_to_string_fix/source"
	_ "github.com/origadmin/abgen/testdata/06_regression/map_string_to_string_fix/target"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/06_regression/map_string_to_string_fix/source,alias=source
//go:abgen:package:path=github.com/origadmin/abgen/testdata/06_regression/map_string_to_string_fix/target,alias=target

// Test case for Bug 2: map[string]string -> string conversion
// This tests the conversion from map[string]string fields to string fields
//go:abgen:convert="source=source.MapToStringSource,target=target.MapToStringTarget,direction=both"