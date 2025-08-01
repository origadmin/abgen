package ast

import (
	"fmt"
	"go/ast"
	"go/token"
	"log/slog"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/types"
)

type TypeResolver interface {
	Resolve(typeName string) (types.TypeInfo, error)
	GetImports() map[string]string
	UpdateFromWalker(walker *PackageWalker) error
	GetKnownTypes() map[string]types.TypeInfo
}

type typeResolver struct {
	pkgs    []*packages.Package
	imports map[string]string
	cache   map[string]types.TypeInfo
}

func (r *typeResolver) GetImports() map[string]string {
	return r.imports
}

func (r *typeResolver) SetImports(imports map[string]string) {
	r.imports = imports
}

// 添加通用类型解析方法
func (r *typeResolver) parseTypeSpec(typeSpec *ast.TypeSpec, impPath string) (types.TypeInfo, error) {
	info := types.TypeInfo{
		Name:       typeSpec.Name.Name,
		ImportPath: impPath,
	}

	// 处理类型别名（type Alias = TargetType）
	if typeSpec.Assign != 0 {
		// 递归解析目标类型
		targetType := r.exprToString(typeSpec.Type)
		resolved, err := r.resolveType(targetType)
		if err != nil {
			return info, fmt.Errorf("解析类型别名失败: %w", err)
		}

		// 继承目标类型的字段
		info.Fields = resolved.Fields
		info.ImportPath = resolved.ImportPath
		return info, nil
	}

	// 原有处理逻辑保持不变...
	// 处理结构体类型
	if structType, ok := typeSpec.Type.(*ast.StructType); ok {
		for _, field := range structType.Fields.List {
			if len(field.Names) == 0 {
				continue
			}

			fieldName := field.Names[0].Name
			info.Fields = append(info.Fields, types.StructField{
				Name:     fieldName,
				Type:     r.exprToString(field.Type),
				Exported: ast.IsExported(fieldName),
			})
		}
		return info, nil
	}

	// 修改基础类型别名的处理方式
	if ident, ok := typeSpec.Type.(*ast.Ident); ok {
		// 如果是基础类型别名（如 type MyInt = int）
		resolved, err := r.resolveType(ident.Name)
		if err == nil {
			info.Fields = resolved.Fields
		} else {
			// 基础类型没有字段
			info.Fields = []types.StructField{}
		}
		return info, nil
	}

	// 处理选择器表达式（跨包别名）
	if sel, ok := typeSpec.Type.(*ast.SelectorExpr); ok {
		targetType := r.exprToString(sel)
		resolved, err := r.resolveType(targetType)
		if err != nil {
			return info, err
		}
		info.Fields = resolved.Fields
		info.ImportPath = resolved.ImportPath
		return info, nil
	}

	return info, fmt.Errorf("unsupported type: %T", typeSpec.Type)
}

// 添加本地类型解析方法
func (r *typeResolver) resolveLocalType(typeName string) (types.TypeInfo, error) {
	// 先检查缓存
	if info, ok := r.cache[typeName]; ok {
		return info, nil
	}

	// 添加本地类型解析逻辑
	for _, pkg := range r.pkgs {
		for _, f := range pkg.Syntax {
			for _, decl := range f.Decls {
				genDecl, ok := decl.(*ast.GenDecl)
				if !ok || genDecl.Tok != token.TYPE {
					continue
				}

				for _, spec := range genDecl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok || typeSpec.Name.Name != typeName {
						continue
					}

					// 解析类型并缓存
					info, err := r.parseTypeSpec(typeSpec, "")
					if err != nil {
						return types.TypeInfo{}, err
					}

					r.cache[typeName] = info
					return info, nil
				}
			}
		}
	}

	return types.TypeInfo{}, fmt.Errorf("local type %s not found", typeName)
}

// 修改 resolveType 方法，确保类型别名解析逻辑一致
func (r *typeResolver) resolveType(typeName string) (types.TypeInfo, error) {
	// 检查缓存中是否直接有该类型
	if info, ok := r.cache[typeName]; ok {
		return info, nil
	}

	// 处理带包前缀的类型 (如ent.User)
	parts := strings.Split(typeName, ".")
	if len(parts) > 1 {
		pkgAlias := parts[0]
		typeName := parts[1]
		slog.Info("处理带包前缀的类型", "包别名", pkgAlias, "类型名", typeName)
		return r.resolveImportedType(pkgAlias, typeName)
	}
	// 处理当前包类型
	return r.resolveLocalType(typeName)
}

// 添加表达式解析方法
func (r *typeResolver) exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return r.exprToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + r.exprToString(t.X)
	case *ast.ArrayType:
		return "[]" + r.exprToString(t.Elt)
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s",
			r.exprToString(t.Key),
			r.exprToString(t.Value))
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.StructType:
		return "struct{}"
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// 在ConverterGenerator中添加
func (r *typeResolver) parseStructType(typeSpec *ast.TypeSpec, impPath string) (types.TypeInfo, error) {
	info := types.TypeInfo{
		Name:       typeSpec.Name.Name,
		ImportPath: impPath,
	}

	structType, ok := typeSpec.Type.(*ast.StructType)
	if !ok {
		return info, fmt.Errorf("%s is not a struct type", typeSpec.Name.Name)
	}

	for _, field := range structType.Fields.List {
		// 跳过匿名字段
		if len(field.Names) == 0 {
			continue
		}

		fieldName := field.Names[0].Name
		if !ast.IsExported(fieldName) {
			continue
		}
		fieldType := r.exprToString(field.Type)

		info.Fields = append(info.Fields, types.StructField{
			Name:     fieldName,
			Type:     fieldType,
			Exported: true,
		})
	}
	return info, nil
}
func (r *typeResolver) loadPackage(importPath string) (*packages.Package, error) {
	// 使用go/packages加载包信息
	cfg := &packages.Config{
		Mode: packages.NeedSyntax | packages.NeedName | packages.NeedFiles |
			packages.NeedCompiledGoFiles | packages.NeedImports |
			packages.NeedTypes | packages.NeedTypesInfo |
			packages.NeedModule,
	}
	pkgs, err := packages.Load(cfg, importPath)
	if err != nil {
		return nil, err
	}
	return pkgs[0], nil
}
func (r *typeResolver) resolveImportedType(pkgAlias, typeName string) (types.TypeInfo, error) {

	// 查找导入路径
	impPath, ok := r.imports[pkgAlias]
	if !ok {
		return types.TypeInfo{}, fmt.Errorf("package %s not imported", pkgAlias)
	}
	slog.Debug("开始解析导入类型",
		"pkgAlias", pkgAlias,
		"importPath", impPath,
		"knownImports", r.imports)
	// 加载目标包
	pkg, err := r.loadPackage(impPath)
	if err != nil {
		slog.Info("加载包失败", "包路径", impPath)
		return types.TypeInfo{}, fmt.Errorf("加载包失败: %w", err)
	}

	var foundType *ast.TypeSpec
	packages.Visit([]*packages.Package{pkg}, func(p *packages.Package) bool {
		for _, f := range p.Syntax {
			for _, decl := range f.Decls {
				genDecl, ok := decl.(*ast.GenDecl)
				if !ok || genDecl.Tok != token.TYPE {
					continue
				}

				for _, spec := range genDecl.Specs {
					typeSpec := spec.(*ast.TypeSpec)
					if typeSpec.Name.Name == typeName {
						foundType = typeSpec
						return false // 终止遍历
					}
				}
			}
		}
		return true
	}, nil)

	if foundType == nil {
		return types.TypeInfo{}, fmt.Errorf("类型 %s 未在包 %s 中找到", typeName, impPath)
	}

	slog.Info("发现类型", "包", impPath, "类型", typeName)
	// 处理类型别名和结构体
	if foundType.Assign != 0 {
		// 处理类型别名（type T = other.Type）
		targetType := r.exprToString(foundType.Type)
		slog.Info("处理类型别名", "目标类型", targetType, "包别名", pkgAlias, "类型名", typeName)
		return r.resolveType(targetType) // 递归解析
	}

	// 处理结构体类型
	if _, ok := foundType.Type.(*ast.StructType); ok {
		return r.parseStructType(foundType, impPath)
	}

	// 处理其他类型声明
	return types.TypeInfo{
		Name:       typeName,
		ImportPath: impPath,
		Fields:     []types.StructField{}, // 非结构体类型无字段
	}, nil
}
func (r *typeResolver) Resolve(typeName string) (types.TypeInfo, error) {
	typeInfo, err := r.resolveType(typeName)
	if err != nil {
		return types.TypeInfo{}, err
	}
	if typeInfo.AliasFor != "" {
		// 如果是类型别名，递归解析
		resolved, err := r.Resolve(typeInfo.AliasFor)
		if err != nil {
			return types.TypeInfo{}, err
		}
		typeInfo.Fields = resolved.Fields
		typeInfo.ImportPath = resolved.ImportPath
	}

	return typeInfo, nil
}

// 增强类型别名解析 - 不使用AliasFor字段
func (r *typeResolver) resolveTypeAlias(typeName string) (types.TypeInfo, bool) {
	// 检查缓存中是否直接有该类型
	if info, ok := r.cache[typeName]; ok {
		return info, true
	}

	// 尝试包前缀形式
	for pkgAlias, pkgPath := range r.imports {
		fullName := pkgPath + "." + typeName
		if info, ok := r.cache[fullName]; ok {
			return info, true
		}

		// 尝试别名形式
		alias := pkgAlias + "." + typeName
		if info, ok := r.cache[alias]; ok {
			return info, true
		}
	}

	return types.TypeInfo{}, false
}

func NewResolver(pkgs []*packages.Package) TypeResolver {
	return &typeResolver{
		pkgs:    pkgs,
		imports: make(map[string]string),
		cache:   make(map[string]types.TypeInfo),
	}
}

func (r *typeResolver) UpdateFromWalker(walker *PackageWalker) error {
	// 从walker获取所需信息
	// r.imports = walker.imports
	// 可以获取其他必要信息...
	for k, v := range walker.imports {
		if _, ok := r.imports[k]; !ok {
			r.imports[k] = v
		}
	}
	for k, v := range walker.cacheTypes {
		if _, ok := r.cache[k]; !ok {
			r.cache[k] = v
		}
	}
	return nil
}

// GetKnownTypes 获取已知类型的映射
func (r *typeResolver) GetKnownTypes() map[string]types.TypeInfo {
	return r.cache
}
