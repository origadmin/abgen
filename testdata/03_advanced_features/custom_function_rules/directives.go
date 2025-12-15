package directives

import (
	_ "github.com/origadmin/abgen/testdata/03_advanced_features/custom_function_rules/source"
	_ "github.com/origadmin/abgen/testdata/03_advanced_features/custom_function_rules/target"
)

//go:abgen:package:path=github.com/origadmin/abgen/testdata/03_advanced_features/custom_function_rules/source,alias=source
//go:abgen:package:path=github.com/origadmin/abgen/testdata/03_advanced_features/custom_function_rules/target,alias=target

//go:abgen:convert="source=source.User,target=target.UserCustom,direction=both"