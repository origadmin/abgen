package pkg_a

//go:abgen:package:path=github.com/origadmin/framework/tools/abgen/testdata/01_dependency_resolving/pkg_b,alias=b
//go:abgen:package:path=github.com/origadmin/framework/tools/abgen/testdata/01_dependency_resolving/pkg_c,alias=c
//go:abgen:pair:packages="b,c"

// TypeA is a type in package A.
type TypeA struct {
	Name string
}
