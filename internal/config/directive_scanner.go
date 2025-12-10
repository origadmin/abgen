package config

import (
	"strings"

	"golang.org/x/tools/go/packages"
)

// DirectiveScanner scans packages for abgen directives and extracts dependencies.
type DirectiveScanner struct{}

// NewDirectiveScanner creates a new DirectiveScanner.
func NewDirectiveScanner() *DirectiveScanner {
	return &DirectiveScanner{}
}

// DiscoverDirectives scans the AST of a package and collects all abgen directives.
func (s *DirectiveScanner) DiscoverDirectives(pkg *packages.Package) []string {
	var directives []string
	const abgenDirectivePrefix = "//go:abgen:"
	
	for _, file := range pkg.Syntax {
		for _, commentGroup := range file.Comments {
			for _, comment := range commentGroup.List {
				if strings.HasPrefix(comment.Text, abgenDirectivePrefix) {
					directives = append(directives, comment.Text)
				}
			}
		}
	}
	return directives
}

// ExtractDependencies parses directives to find all referenced package paths.
func (s *DirectiveScanner) ExtractDependencies(directives []string) []string {
	depMap := make(map[string]struct{})
	for _, d := range directives {
		if strings.Contains(d, "path=") {
			parts := strings.Split(d, "path=")
			if len(parts) > 1 {
				pathPart := parts[1]
				path := strings.Split(pathPart, ",")[0]
				depMap[path] = struct{}{}
			}
		}
	}

	deps := make([]string, 0, len(depMap))
	for dep := range depMap {
		deps = append(deps, dep)
	}
	return deps
}