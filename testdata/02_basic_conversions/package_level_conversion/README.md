# Package-Level Conversion Test (Identical Names)

## Test Goal
Validate abgen's ability to automatically generate bidirectional conversion functions between structs in different packages that have identical type names, using the `pair:packages` directive. This test also verifies that generated functions correctly use pointer parameters.

## Test Scenario
This test case involves two source structs (`source.User`, `source.Item`) and their corresponding target structs (`target.User`, `target.Item`) located in separate packages (`source` and `target`). The `//go:abgen:pair:packages` directive is used to instruct `abgen` to find all identically named types in both packages and generate bidirectional conversion functions for them.

## Input Data
The input consists of `directive.go` with the following abgen directives:

```go
//go:abgen:package:path=github.com/origadmin/abgen/testdata/02_basic_conversions/package_level_conversion/source,alias=source
//go:abgen:package:path=github.com/origadmin/abgen/testdata/02_basic_conversions/package_level_conversion/target,alias=target

//go:abgen:pair:packages=source,target
```

## Expected Output
`abgen` should generate a `package_level_conversion.gen.go` file containing four conversion functions:
- `ItemTarget = target.Item`
- `ItemSource = source.Item`
- `UserSource = source.User`
- `UserTarget = target.User`
- `ConvertItemSourceToItemTarget(from *ItemSource) *ItemTarget`
- `ConvertItemTargetToItemSource(from *ItemTarget) *ItemSource`
- `ConvertUserSourceToUserTarget(from *UserSource) *UserTarget`
- `ConvertUserTargetToUserSource(from *UserTarget) *UserSource`

Each function should correctly map fields between the source and target types and include a `nil` check for the source pointer.

## Validation Points
- [x] `package_level_conversion.gen.go` is generated.
- [x] `ConvertItemSourceToItemTarget` function is generated, using pointer parameters.
- [x] `ConvertItemTargetToItemSource` function is generated, using pointer parameters.
- [x] `ConvertUserSourceToUserTarget` function is generated, using pointer parameters.
- [x] `ConvertUserTargetToUserSource` function is generated, using pointer parameters.
- [x] Each generated function includes a `nil` check for the source pointer.
- [x] Fields `ID` and `Name` are correctly mapped for both `User`/`User` and `Item`/`Item` conversions.

## Related Documentation
- [01_directives.md](../../../docs/01_directives.md)
- [FIX_PLAN.md](../../../FIX_PLAN.md)
- [TESTDATA_RESTRUCTURE_PLAN.md](../../../TESTDATA_RESTRUCTURE_PLAN.md)

## Status Record
- Initial review: Identified that the original `pair:packages` directive would not generate conversions due to differing type names. Modified `target/user_dto.go` to rename types for identical name matching. Reverted `directive.go` to solely use `pair:packages`. Confirmed pointer parameter generation and naming convention with default suffixes.