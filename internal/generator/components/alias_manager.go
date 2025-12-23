package components

import (
	"log/slog"
	"regexp"
	"strings"
	"unicode"

	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

var _ model.AliasManager = (*AliasManager)(nil)

var nonManagedPackages = map[string]struct{}{
	"time":                            {},
	"encoding/json":                   {},
	"github.com/google/uuid":          {},
	"google.golang.org/protobuf/types/known/timestamppb": {},
	"google.golang.org/protobuf/types/known/durationpb":  {},
	"google.golang.org/protobuf/types/known/structpb":    {},
}

type AliasManager struct {
	cfg                 *config.Config
	importManager       model.ImportManager
	aliasMap            map[string]string
	typeInfos           map[string]*model.TypeInfo
	aliasedTypes        map[string]*model.TypeInfo
	managedPackagePaths map[string]struct{}
	fqnToExistingAlias  map[string]string
	visited             map[string]bool
}

var camelCaseRegexp = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func NewAliasManager(
	analysisResult *model.AnalysisResult,
	importManager model.ImportManager,
) model.AliasManager {
	fqnToAlias := make(map[string]string, len(analysisResult.ExistingAliases))
	for alias, fqn := range analysisResult.ExistingAliases {
		fqnToAlias[fqn] = alias
	}

	return &AliasManager{
		cfg:                 analysisResult.ExecutionPlan.FinalConfig,
		importManager:       importManager,
		aliasMap:            make(map[string]string),
		typeInfos:           analysisResult.TypeInfos,
		aliasedTypes:        make(map[string]*model.TypeInfo),
		managedPackagePaths: make(map[string]struct{}),
		fqnToExistingAlias:  fqnToAlias,
		visited:             make(map[string]bool),
	}
}

func (am *AliasManager) IsUserDefined(uniqueKey string) bool {
	_, ok := am.fqnToExistingAlias[uniqueKey]
	return ok
}

func (am *AliasManager) GetAliasedTypes() map[string]*model.TypeInfo {
	return am.aliasedTypes
}

func (am *AliasManager) GetAlias(info *model.TypeInfo) (string, bool) {
	if info == nil {
		return "", false
	}
	return am.LookupAlias(info.UniqueKey())
}

func (am *AliasManager) LookupAlias(uniqueKey string) (string, bool) {
	alias, ok := am.aliasMap[uniqueKey]
	return alias, ok
}

func (am *AliasManager) PopulateAliases() {
	// Correctly populate managed packages by inspecting the TypeInfo of the rule types.
	for _, rule := range am.cfg.ConversionRules {
		sourceInfo := am.typeInfos[rule.SourceType]
		targetInfo := am.typeInfos[rule.TargetType]

		if sourceInfo != nil {
			elem := model.GetElementType(sourceInfo)
			if elem != nil {
				am.addManagedPackage(elem.ImportPath)
			}
		}
		if targetInfo != nil {
			elem := model.GetElementType(targetInfo)
			if elem != nil {
				am.addManagedPackage(elem.ImportPath)
			}
		}
	}
	slog.Debug("AliasManager: collected managed packages for aliasing", "paths", am.managedPackagePaths)

	for _, rule := range am.cfg.ConversionRules {
		sourceInfo := am.typeInfos[rule.SourceType]
		targetInfo := am.typeInfos[rule.TargetType]
		if sourceInfo != nil {
			am.ensureAliasesRecursively(sourceInfo, true)
		}
		if targetInfo != nil {
			am.ensureAliasesRecursively(targetInfo, false)
		}
	}
}

func (am *AliasManager) addManagedPackage(pkgPath string) {
	if pkgPath == "" {
		return
	}
	if _, isExcluded := nonManagedPackages[pkgPath]; isExcluded {
		return
	}
	am.managedPackagePaths[pkgPath] = struct{}{}
}

func (am *AliasManager) ensureAliasesRecursively(typeInfo *model.TypeInfo, isSource bool) {
	if typeInfo == nil {
		return
	}

	uniqueKey := typeInfo.UniqueKey()
	if am.visited[uniqueKey] {
		return
	}
	am.visited[uniqueKey] = true

	if typeInfo.Kind == model.Pointer {
		am.ensureAliasesRecursively(typeInfo.Underlying, isSource)
		return
	}

	switch typeInfo.Kind {
	case model.Struct:
		for _, field := range typeInfo.Fields {
			am.ensureAliasesRecursively(field.Type, isSource)
		}
	case model.Slice, model.Array:
		am.ensureAliasesRecursively(typeInfo.Underlying, isSource)
	case model.Map:
		am.ensureAliasesRecursively(typeInfo.KeyType, isSource)
		am.ensureAliasesRecursively(typeInfo.Underlying, isSource)
	}

	if !am.isManagedType(typeInfo) {
		return
	}

	if existingAlias, ok := am.fqnToExistingAlias[uniqueKey]; ok {
		am.aliasMap[uniqueKey] = existingAlias
	} else {
		alias := am.generateAlias(typeInfo, isSource)
		am.aliasMap[uniqueKey] = alias
		am.aliasedTypes[uniqueKey] = typeInfo
	}
}

func (am *AliasManager) isManagedType(info *model.TypeInfo) bool {
	if info == nil {
		return false
	}
	elem := model.GetElementType(info)
	if elem == nil {
		return false
	}
	switch elem.Kind {
	case model.Named, model.Struct:
		_, isManaged := am.managedPackagePaths[elem.ImportPath]
		return isManaged
	default:
		return false
	}
}

func (am *AliasManager) getRecursiveBaseName(info *model.TypeInfo) string {
	if info == nil {
		return ""
	}
	switch info.Kind {
	case model.Pointer:
		return am.getRecursiveBaseName(info.Underlying)
	case model.Slice:
		elementBaseName := am.getRecursiveBaseName(info.Underlying)
		if strings.HasSuffix(elementBaseName, "s") {
			return elementBaseName + "es"
		}
		return elementBaseName + "s"
	case model.Array:
		return am.getRecursiveBaseName(info.Underlying) + "Array"
	case model.Map:
		keyBaseName := am.getRecursiveBaseName(info.KeyType)
		valueBaseName := am.getRecursiveBaseName(info.Underlying)
		return keyBaseName + "To" + valueBaseName + "Map"
	case model.Named, model.Struct:
		return am.toCamelCase(info.Name)
	case model.Primitive:
		return am.toCamelCase(info.Name)
	default:
		return "Unknown"
	}
}

func (am *AliasManager) generateAlias(info *model.TypeInfo, isSource bool) string {
	baseName := am.getRecursiveBaseName(info)
	prefix, suffix := am.getPrefixAndSuffix(isSource)
	return am.toCamelCase(prefix) + baseName + am.toCamelCase(suffix)
}

func (am *AliasManager) getPrefixAndSuffix(isSource bool) (string, string) {
	if isSource {
		return am.cfg.NamingRules.SourcePrefix, am.cfg.NamingRules.SourceSuffix
	}
	return am.cfg.NamingRules.TargetPrefix, am.cfg.NamingRules.TargetSuffix
}

func (am *AliasManager) toCamelCase(s string) string {
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

func (am *AliasManager) GetAllAliases() map[string]string { return am.aliasMap }
func (am *AliasManager) GetSourcePath() string {
	if len(am.cfg.PackagePairs) > 0 {
		return am.cfg.PackagePairs[0].SourcePath
	}
	return ""
}
func (am *AliasManager) GetTargetPath() string {
	if len(am.cfg.PackagePairs) > 0 {
		return am.cfg.PackagePairs[0].TargetPath
	}
	return ""
}
func getPkgPath(fqn string) string {
	lastDot := strings.LastIndex(fqn, ".")
	if lastDot == -1 {
		return ""
	}
	return fqn[:lastDot]
}
