package generator

import (
	"fmt"
	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/model"
)

// Namer is responsible for generating names for functions and type aliases.
type Namer struct {
	config   *config.Config
	aliasMap map[string]string
}

// NewNamer creates a new Namer.
func NewNamer(config *config.Config, aliasMap map[string]string) *Namer {
	return &Namer{
		config:   config,
		aliasMap: aliasMap,
	}
}

// GetFunctionName generates a name for a conversion function.
func (n *Namer) GetFunctionName(source, target *model.TypeInfo) string {
	sourceName := n.getAliasedOrBaseName(source)
	targetName := n.getAliasedOrBaseName(target)
	return fmt.Sprintf("Convert%sTo%s", sourceName, targetName)
}

// GetAlias generates a type alias based on the naming rules.
func (n *Namer) GetAlias(info *model.TypeInfo, isSource, disambiguate bool) string {
	baseName := info.Name
	
	// Check if a user-defined naming rule applies
	hasSpecificRule := (isSource && (n.config.NamingRules.SourcePrefix != "" || n.config.NamingRules.SourceSuffix != "")) ||
		(!isSource && (n.config.NamingRules.TargetPrefix != "" || n.config.NamingRules.TargetSuffix != ""))

	var prefix, suffix string
	if isSource {
		prefix = n.config.NamingRules.SourcePrefix
		suffix = n.config.NamingRules.SourceSuffix
		// Only add "Source" for disambiguation if no specific rule for the pair exists.
		if !hasSpecificRule && disambiguate {
			suffix = "Source"
		}
	} else { // is Target
		prefix = n.config.NamingRules.TargetPrefix
		suffix = n.config.NamingRules.TargetSuffix
		// Only add "Target" for disambiguation if no specific rule for the pair exists.
		if !hasSpecificRule && disambiguate {
			suffix = "Target"
		}
	}

	return prefix + baseName + suffix
}

// getAliasedOrBaseName returns the alias if it exists, otherwise returns the base name.
func (n *Namer) getAliasedOrBaseName(info *model.TypeInfo) string {
	fqn := info.FQN()
	if fqn == "" {
		fqn = info.Name // Handle primitives
	}
	if alias, ok := n.aliasMap[fqn]; ok {
		return alias
	}
	return info.Name
}
