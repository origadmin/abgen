package model

import (
	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/config"
)

// Analyzer defines the contract for the type analysis phase.
// It takes a configuration and returns a fully resolved type map.
type Analyzer interface {
	Analyze(cfg *config.Config) (map[string]*TypeInfo, error)
}

// Parser defines the contract for the configuration parsing phase.
// It discovers and parses directives from a given package.
type Parser interface {
	ParseDirectives(pkg *packages.Package) (*config.Config, error)
}
