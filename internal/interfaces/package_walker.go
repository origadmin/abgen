package interfaces

import (
	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/analyzer"
)

// PackageWalker defines the contract for package analysis and type resolution.
type PackageWalker interface {
	// LoadInitialPackage loads only the essential information for a single package.
	LoadInitialPackage(path string) (*packages.Package, error)
	
	// LoadFullGraph loads the complete type information for the initial package
	// and all its specified dependencies.
	LoadFullGraph(initialPath string, dependencyPaths ...string) ([]*packages.Package, error)
	
	// FindTypeByFQN finds a type by its fully qualified name.
	FindTypeByFQN(fqn string) (*analyzer.TypeInfo, error)
	
	// GetLoadedPackages returns the list of loaded packages.
	GetLoadedPackages() []*packages.Package
	
	// GetFailedPackagesCount returns the count of failed package loads.
	GetFailedPackagesCount() int
}