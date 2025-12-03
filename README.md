# abgen - Alias Binding Generator

`abgen` is a command-line tool that automatically generates Go code for converting between two different struct types. It is highly configurable and designed to reduce boilerplate code, especially when converting between data models (DO), API models (VO), and protobuf-generated types (PB).

## Features

- **Type-based Conversion**: Generate converters by annotating a struct.
- **Package-based Conversion**: Generate converters for all matching types between two entire packages.
- Configurable conversion direction (`to`, `from`, `both`).
- Selective ignoring of types or fields.

## Usage

`abgen` is controlled via `//go:abgen:...` directives in your Go source files.

### Command

To run the generator, execute the following command from your project's root directory:

```shell
go run ./tools/abgen/cmd/abgen <path-to-directory-with-directives>
```

The generated code will be placed in a `.gen.go` file inside the target directory.

### Example 1: Package-Level Conversion

This is the most powerful feature. It allows you to generate converters for all matching types between two external packages.

1.  **Add Directives**: In the package where you want the generated code to live, create a `.go` file (e.g., `provider.go`) and add file-level directives at the top.

    *File: `internal/service/provider.go`*
    ```go
    // The following directives instruct abgen to generate converters for all matching types
    // between the data model package and the API package.

    //go:abgen:convert-package:source="github.com/your/project/internal/data/po"
    //go:abgen:convert-package:target="github.com/your/project/api/v1/system"
    //go:abgen:convert-package:direction=to
    //go:abgen:convert-package:ignore=InternalType,TimestampMixin

    package service

    // ... rest of your code
    ```

2.  **Run `abgen`**:
    ```shell
    go run ./tools/abgen/cmd/abgen ./internal/service
    ```

3.  **Result**: A `service.gen.go` file will be created in `internal/service`, containing converter functions for all types that exist in both the `po` and `system` packages (except for the ignored ones).

### Example 2: Type-Level Conversion

For one-off conversions or to override package-level behavior, you can add a directive directly to a struct.

1.  **Add Directive**:
    ```go
    package dto

    //go:abgen:convert:target="UserPB"
    //go:abgen:convert:ignore="Password"
    type User struct {
        ID       int
        Username string
        Password string
    }
    ```

2.  **Run `abgen`**:
    ```shell
    go run ./tools/abgen/cmd/abgen ./internal/dto
    ```

3.  **Result**: A `dto.gen.go` file will be created containing the `UserToUserPB` and `UserPBToUser` functions.


## Directive Reference

### Package-Level Directives

*   `//go:abgen:convert-package:source="<import-path>"`: (Required) Sets the full import path for the source package.
*   `//go:abgen:convert-package:target="<import-path>"`: (Required) Sets the full import path for the target package.
*   `//go:abgen:convert-package:direction="<to|from|both>"`: (Optional) Sets the conversion direction. Defaults to `both`.
*   `//go:abgen:convert-package:ignore="<TypeName1,TypeName2>"`: (Optional) A comma-separated list of types to ignore during generation.

### Type-Level Directives

*   `//go:abgen:convert:target="<TypeName>"`: (Required) The name of the target type to convert to/from.
*   `//go:abgen:convert:direction="<to|from|both>"`: (Optional) The conversion direction for this specific type.
*   `//go:abgen:convert:ignore="<FieldName1,FieldName2>"`: (Optional) A comma-separated list of fields to ignore during conversion.