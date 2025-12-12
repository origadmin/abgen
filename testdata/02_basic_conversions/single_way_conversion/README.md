# Single-Way Conversion Test

## Test Goal
Validate abgen's ability to generate a single-direction conversion function between two simple struct types, using the `direction=oneway` rule. This test also verifies that the generated function correctly uses pointer parameters.

## Test Scenario
This test case involves a simple source struct (`source.User`) and its corresponding target DTO (`target.UserDTO`) located in separate packages. The `//go:abgen:convert` directive is used to define a conversion only from `source.User` to `target.UserDTO` (`direction=oneway`).

## Input Data
The input consists of `directive.go` with the following abgen directives:

```go
//go:abgen:package:path=github.com/origadmin/abgen/testdata/02_basic_conversions/single_way_conversion/source,alias=source
//go:abgen:package:path=github.com/origadmin/abgen/testdata/02_basic_conversions/single_way_conversion/target,alias=target

//go:abgen:convert="source=source.User,target=target.UserDTO,direction=oneway"
```

## Expected Output
`abgen` should generate a `single_way_conversion.gen.go` file containing only one conversion function:
- `ConvertUserToUserDTO(from *User) *UserDTO`

The function should correctly map fields between the source and target types and include a `nil` check for the source pointer. No `ConvertUserDTOToUser` function should be generated.

## Validation Points
- [x] `single_way_conversion.gen.go` is generated.
- [x] Only `ConvertUserToUserDTO` function is generated.
- [x] `ConvertUserToUserDTO` function uses pointer parameters.
- [x] The generated function includes a `nil` check for the source pointer.
- [x] Fields `ID` and `Name` are correctly mapped for the `User` to `UserDTO` conversion.
- [ ] No `ConvertUserDTOToUser` function is generated.

## Related Documentation
- [01_directives.md](../../../docs/01_directives.md)
- [FIX_PLAN.md](../../../FIX_PLAN.md)
- [TESTDATA_RESTRUCTURE_PLAN.md](../../../TESTDATA_RESTRUCTURE_PLAN.md)

## Status Record
- Initial setup: Created source/target structs and `directive.go` with `direction=oneway`.
- Confirmed pointer parameter generation and single-way conversion with test run.
