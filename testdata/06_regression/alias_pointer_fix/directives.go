//go:generate go run github.com/origadmin/abgen/cmd/abgen -debug -output ./expected.golden .
package regression

//go:abgen:package:path=github.com/origadmin/abgen/testdata/06_regression/alias_pointer_fix/source,alias=source
//go:abgen:package:path=github.com/origadmin/abgen/testdata/06_regression/alias_pointer_fix/target,alias=target

// --- Restoring Final Strategy ---
// With the core logic in generator.go now fixed, we use the correct directive
// format with fully qualified names (FQNs) to ensure the test passes.

//go:abgen:convert="source=source.User,target=target.User"
//go:abgen:convert="source=source.Address,target=target.Address"
