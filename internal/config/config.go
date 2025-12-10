package config

// Application constants
const (
	Application = "abgen"
	Description = "Auto generate conversion code between structs"
	WebSite     = "https://github.com/origadmin/abgen"
	UI          = "abgen"
)

// ConversionConfig holds the overall configuration for the conversion process.
type ConversionConfig struct {
	// SourcePackages holds the source package configurations.
	SourcePackages []*PackageConfig
	// TargetPackages holds the target package configurations.
	TargetPackages []*PackageConfig
	// Pairs holds the source-target pair configurations.
	Pairs []*PairConfig
}

// PackageConfig holds configuration for a single package.
type PackageConfig struct {
	Path  string
	Alias string
}

// PairConfig holds configuration for a source-target pair.
type PairConfig struct {
	Source      *PackageConfig
	Target      *PackageConfig
	RuleSet     *RuleSet
	Direction   ConversionDirection
	IgnoreTypes []string
}

// NewDefaultConfig creates a default configuration.
func NewDefaultConfig() *ConversionConfig {
	return &ConversionConfig{
		SourcePackages: make([]*PackageConfig, 0),
		TargetPackages: make([]*PackageConfig, 0),
		Pairs:          make([]*PairConfig, 0),
	}
}