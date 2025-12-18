package components

import (
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

// ConcreteNameGenerator 实现 NameGenerator 接口
type ConcreteNameGenerator struct {
	config     *config.Config
	aliasMap   map[string]string
	sourcePkgs map[string]struct{}
}

var camelCaseRegexp = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// NewNameGenerator 创建新的命名生成器
func NewNameGenerator(config *config.Config, aliasMap map[string]string) model.NameGenerator {
	return &ConcreteNameGenerator{
		config:     config,
		aliasMap:   aliasMap,
		sourcePkgs: make(map[string]struct{}),
	}
}

// PopulateSourcePkgs 从最终配置中填充源包映射
// 这应该在所有隐式规则被发现后调用
func (n *ConcreteNameGenerator) PopulateSourcePkgs(config *config.Config) {
	// 1. 从显式的 PackagePairs 填充
	for _, pair := range config.PackagePairs {
		n.sourcePkgs[pair.SourcePath] = struct{}{}
	}

	// 2. 从 ConversionRules 的 SourceType 填充（如果 PackagePairs 不完整）
	for _, rule := range config.ConversionRules {
		// rule.SourceType 是一个像 "pkg/path.TypeName" 的 FQN
		lastDot := strings.LastIndex(rule.SourceType, ".")
		if lastDot != -1 {
			pkgPath := rule.SourceType[:lastDot]
			n.sourcePkgs[pkgPath] = struct{}{}
		}
	}
}

// toCamelCase 将字符串转换为驼峰命名
func (n *ConcreteNameGenerator) toCamelCase(s string) string {
	if s == "" {
		return ""
	}
	s = camelCaseRegexp.ReplaceAllString(s, " ")
	parts := strings.Fields(s)
	for i := range parts {
		if len(parts[i]) > 0 {
			runes := []rune(parts[i])
			runes[0] = unicode.ToUpper(runes[0])
			parts[i] = string(runes)
		}
	}
	return strings.Join(parts, "")
}

// GetFunctionName 为转换函数生成名称
func (n *ConcreteNameGenerator) GetFunctionName(source, target *model.TypeInfo) string {
	sourceName := n.getAliasedOrBaseName(source)
	targetName := n.getAliasedOrBaseName(target)
	return fmt.Sprintf("Convert%sTo%s", sourceName, targetName)
}

// GetPrimitiveConversionStubName 为基本转换存根函数生成名称
func (n *ConcreteNameGenerator) GetPrimitiveConversionStubName(
	parentSource *model.TypeInfo, sourceField *model.FieldInfo,
	parentTarget *model.TypeInfo, targetField *model.FieldInfo,
) string {
	parentSourceName := n.getAliasedOrBaseName(parentSource)
	parentTargetName := n.getAliasedOrBaseName(parentTarget)
	sourceFieldName := n.toCamelCase(sourceField.Name)
	targetFieldName := n.toCamelCase(targetField.Name)
	return fmt.Sprintf("Convert%s%sTo%s%s",
		parentSourceName, sourceFieldName,
		parentTargetName, targetFieldName,
	)
}

// getPrefixAndSuffix 返回为源类型或目标类型配置的前缀和后缀
func (n *ConcreteNameGenerator) getPrefixAndSuffix(isSource bool) (prefix string, suffix string) {
	slog.Debug("getPrefixAndSuffix",
		"isSource", isSource,
		"SourcePrefix", n.config.NamingRules.SourcePrefix,
		"SourceSuffix", n.config.NamingRules.SourceSuffix,
		"TargetPrefix", n.config.NamingRules.TargetPrefix,
		"TargetSuffix", n.config.NamingRules.TargetSuffix,
	)
	if isSource {
		return n.config.NamingRules.SourcePrefix, n.config.NamingRules.SourceSuffix
	}
	return n.config.NamingRules.TargetPrefix, n.config.NamingRules.TargetSuffix
}

// getRawTypeName 从 TypeInfo 中提取基础名称
// 优先使用 TypeInfo 自己的 Name 字段
// 如果 TypeInfo 是未命名的容器/指针，则递归查找底层的命名类型
func (n *ConcreteNameGenerator) getRawTypeName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}

	// 如果类型本身有名称，则使用它作为基础名称
	// 这涵盖了结构体、基本类型和命名容器类型（例如 `type MyList []int`）
	if info.Name != "" {
		return info.Name
	}

	// 如果是未命名的容器或指针，递归查找底层的命名类型
	switch info.Kind {
	case model.Slice, model.Array, model.Pointer, model.Map:
		if info.Underlying != nil {
			return n.getRawTypeName(info.Underlying)
		}
	}
	// 对于没有名称且没有底层的类型的回退（例如，空的 TypeInfo 或名称为空的基本类型）
	return info.Name // 如果上面的 info.Name 为空，这里将返回空
}

// GetAlias 根据配置的命名规则为给定的 TypeInfo 生成合适的别名
func (n *ConcreteNameGenerator) GetAlias(info *model.TypeInfo, isSource bool) string {
	if info.Kind == model.Primitive {
		return n.toCamelCase(info.Name)
	}

	prefix, suffix := n.getPrefixAndSuffix(isSource)
	rawBaseName := n.getRawTypeName(info) // 例如，从 "[]*User" 中提取 "User"，或从 "type UserList []User" 中提取 "UserList"

	// 1. 将原始基础名称转换为驼峰命名
	processedBaseName := n.toCamelCase(rawBaseName) // 例如，"User" 或 "UserList"

	// 2. 为容器类型应用智能默认值作为类型指示器
	// 这在全局前缀/后缀之前发生
	switch info.Kind {
	case model.Slice, model.Array:
		// 无条件附加 's' 作为切片/数组的类型指示器以确保唯一性
		// 如果基础名称已经是 "Users"，这可能导致 "Userss" 之类的名称
		// 但它保证了与单数结构体 "Users" 的不同性
		processedBaseName += "s"
	case model.Map:
		processedBaseName += "Map" // 例如，"UserMap"
	}

	// 3. 将全局前缀和后缀应用到处理后的名称
	finalAlias := n.toCamelCase(prefix) + processedBaseName + n.toCamelCase(suffix)

	return finalAlias
}

// GetTypeString 生成 TypeInfo 的完整字符串表示，
// 包括指针、切片、数组和映射，并处理包路径
// 这用于类型别名定义的右侧
func (n *ConcreteNameGenerator) GetTypeString(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}

	var sb strings.Builder

	switch info.Kind {
	case model.Pointer:
		sb.WriteString("*")
		if info.Underlying != nil {
			sb.WriteString(n.GetTypeString(info.Underlying))
		}
	case model.Slice:
		sb.WriteString("[]")
		if info.Underlying != nil {
			// 如果切片的底层类型最终是结构体，它在切片中应该是指针
			if info.Underlying.IsUltimatelyStruct() {
				sb.WriteString("*")
			}
			sb.WriteString(n.GetTypeString(info.Underlying))
		}
	case model.Array:
		sb.WriteString(fmt.Sprintf("[%d]", info.ArrayLen)) // 使用 ArrayLen
		if info.Underlying != nil {
			if info.Underlying.IsUltimatelyStruct() {
				sb.WriteString("*")
			}
			sb.WriteString(n.GetTypeString(info.Underlying))
		}
	case model.Map:
		sb.WriteString("map[")
		if info.KeyType != nil { // 使用 KeyType
			sb.WriteString(n.GetTypeString(info.KeyType))
		} else {
			sb.WriteString("interface{}") // 如果 KeyType 未设置，则回退
		}
		sb.WriteString("]")
		if info.Underlying != nil { // Underlying 是值类型
			if info.Underlying.IsUltimatelyStruct() {
				sb.WriteString("*")
			}
			sb.WriteString(n.GetTypeString(info.Underlying))
		}
	default: // model.Struct, model.Primitive 等
		// 使用 BuildQualifiedTypeName 正确处理包路径
		info.BuildQualifiedTypeName(&sb)
	}

	return sb.String()
}

// GetTypeAliasString 如果存在别名，则获取类型的别名化名称，否则返回完整类型字符串
// 这是生成应使用本地别名的代码时的主要方法
func (n *ConcreteNameGenerator) GetTypeAliasString(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}
	// 首先，检查精确类型的别名是否存在
	if alias, ok := n.aliasMap[info.UniqueKey()]; ok && alias != "" {
		return alias
	}

	// 如果没有找到别名，则意味着它可能是基本类型、当前包的类型
	// 或者不需要别名的其他类型。在这种情况下，我们构造其完整类型字符串
	return n.GetTypeString(info)
}

// getAliasedOrBaseName 如果映射中存在别名，则返回别名，否则返回简单名称
func (n *ConcreteNameGenerator) getAliasedOrBaseName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}
	if alias, ok := n.aliasMap[info.UniqueKey()]; ok && alias != "" {
		slog.Debug("getAliasedOrBaseName: found cached alias", "fqn", info.FQN(), "alias", alias)
		return alias
	}

	// 递归查找基础命名类型以确定包路径
	// 这对于切片和指针等复合类型至关重要，它们自己的 ImportPath 为空
	baseType := info
	for baseType != nil && (baseType.Kind == model.Slice || baseType.Kind == model.Array || baseType.Kind == model.Pointer) {
		if baseType.Underlying == nil {
			// 如果不能更深入，则停止以防止空指针解引用
			break
		}
		baseType = baseType.Underlying
	}

	// 使用基础类型的包路径确定逻辑角色
	var isSource bool
	if baseType != nil {
		_, isSource = n.sourcePkgs[baseType.ImportPath]
	}

	// 用于调试：打印 sourcePkgs 键
	sourcePkgKeys := make([]string, 0, len(n.sourcePkgs))
	for k := range n.sourcePkgs {
		sourcePkgKeys = append(sourcePkgKeys, k)
	}
	sort.Strings(sourcePkgKeys) // 排序以保持一致的日志顺序

	slog.Debug("getAliasedOrBaseName: determining isSource",
		"original_fqn", info.FQN(),
		"baseType_fqn", baseType.FQN(),
		"baseType_importPath", baseType.ImportPath,
		"sourcePkgs_keys", sourcePkgKeys,
		"determined_isSource", isSource,
	)

	// 使用原始类型 'info' 调用 GetAlias，但使用正确确定的 'isSource' 标志
	return n.GetAlias(info, isSource)
}