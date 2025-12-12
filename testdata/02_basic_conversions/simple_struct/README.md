# Simple Struct Conversion Test

## Test Goal
Validate the basic functionality of abgen to generate bidirectional conversion functions between two simple struct types, including field remapping.

## Test Scenario
This test case involves two simple structs, `source.User` and `target.UserDTO`, located in separate packages. The `abgen` directive specifies a bidirectional conversion (`direction=both`) and a field remapping rule (`remap=Name:UserName`) to convert the `Name` field in `source.User` to `UserName` in `target.UserDTO`.

## Input Data
The input consists of `directive.go` with the following abgen directives:

```go
//go:abgen:package:path=github.com/origadmin/abgen/testdata/02_basic_conversions/simple_struct/source,alias=source
//go:abgen:package:path=github.com/origadmin/abgen/testdata/02_basic_conversions/simple_struct/target,alias=target

//go:abgen:convert="source=source.User,target=target.UserDTO,direction=both,remap=Name:UserName"
```

## Expected Output
`abgen` should generate a `simple_struct.gen.go` file containing two conversion functions:
- `ConvertUserToUserDTO(source User) UserDTO`: Converts `source.User` to `target.UserDTO`, mapping `source.Name` to `target.UserName`.
- `ConvertUserDTOToUser(source UserDTO) User`: Converts `target.UserDTO` back to `source.User`, mapping `target.UserName` to `source.Name`.

The generated functions should not use pointer parameters based on the current implementation.

## Validation Points
- [x] `simple_struct.gen.go` is generated.
- [x] `ConvertUserToUserDTO` function is generated.
- [x] `ConvertUserDTOToUser` function is generated.
- [x] `ID` field is correctly mapped in both directions.
- [x] `Name` field from `source.User` is correctly remapped to `UserName` in `target.UserDTO` for `ConvertUserToUserDTO`.
- [x] `UserName` field from `target.UserDTO` is correctly remapped to `Name` in `source.User` for `ConvertUserDTOToUser`.
- [ ] Generated functions use value parameters, not pointer parameters (as per current implementation behavior, to be changed later).

## Related Documentation
- [01_directives.md](../../../docs/01_directives.md)
- [FIX_PLAN.md](../../../FIX_PLAN.md)
- [TESTDATA_RESTRUCTURE_PLAN.md](../../../TESTDATA_RESTRUCTURE_PLAN.md)

## Status Record
- Initial review: Checked generated output against expected.golden. Found that generated functions use value parameters, which is currently expected but will be changed according to FIX_PLAN.md.
