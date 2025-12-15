//go:build abgen_source

package directives

// Define package aliases to be used in the 'convert' rules below.
//go:abgen:package:path="github.com/origadmin/abgen/testdata/02_basic_conversions/slice_conversion/source,alias=source"
//go:abgen:package:path="github.com/origadmin/abgen/testdata/02_basic_conversions/slice_conversion/target,alias=target"

//go:abgen:pair:packages="source,target"
//go:abgen:convert:source:suffix="Source"
//go:abgen:convert:target:suffix="Target"
