package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	goversion "github.com/caarlos0/go-version"
	"golang.org/x/tools/go/packages"

	"github.com/origadmin/abgen/internal/analyzer"
	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/generator"
)

var (
	version   = "0.0.1"
	commit    = ""
	treeState = ""
	date      = ""
	builtBy   = ""
	debug     = flag.Bool("debug", false, "Enable debug logging")
	output    = flag.String("output", "", "Output file name for the main generated code. Defaults to <package_name>.gen.go.")
	customOutput = flag.String("custom-output", "custom.gen.go", "Output file name for custom conversion stubs.")
	logFile   = flag.String("log-file", "", "Path to a file where logs should be written. If empty, logs go to stderr.") // Added log-file flag
)

func main() {
	flag.Parse()

	// Configure log output
	var logWriter *os.File
	if *logFile != "" {
		var err error
		logWriter, err = os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			slog.Error("Failed to open log file", "file", *logFile, "error", err)
			os.Exit(1)
		}
		defer logWriter.Close()
	} else {
		logWriter = os.Stderr
	}

	logLevel := slog.LevelWarn // Changed default log level to Warn
	if *debug {
		logLevel = slog.LevelDebug
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{
		Level: logLevel,
	})))

	if len(flag.Args()) == 0 {
		v := buildVersion(version, commit, date, builtBy, treeState)
		fmt.Println(v.String())
		fmt.Println("Usage: abgen [options] <source_directory>")
		flag.PrintDefaults()
		return
	}

	sourceDir := flag.Arg(0)
	slog.Info("Starting abgen", "sourceDir", sourceDir)

	// --- 1. Load the initial package to find directives ---
	slog.Debug("Loading initial package...")
	directiveParser := config.NewDirectiveParser()
	
	initialLoaderCfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedSyntax | packages.NeedFiles,
		Dir:  sourceDir,
		Tests: false,
	}
	initialPkgs, err := packages.Load(initialLoaderCfg, ".")
	if err != nil {
		slog.Error("Failed to load initial package for directives", "error", err)
		os.Exit(1)
	}
	if packages.PrintErrors(initialPkgs) > 0 {
		slog.Error("Initial package for directives contains errors")
		os.Exit(1)
	}
	if len(initialPkgs) == 0 {
		slog.Error("No initial package found at path", "sourceDir", sourceDir)
		os.Exit(1)
	}
	initialPkg := initialPkgs[0]

	// --- 2. Discover and parse directives ---
	slog.Debug("Discovering directives...")
	directives, err := directiveParser.DiscoverDirectives(initialPkg)
	if err != nil {
		slog.Error("Failed to discover directives", "error", err)
		os.Exit(1)
	}
	
	ruleParser := config.NewRuleParser()
	if err := ruleParser.ParseDirectives(directives, initialPkg); err != nil {
		slog.Error("Failed to parse directives", "error", err)
		os.Exit(1)
	}
	ruleSet := ruleParser.GetRuleSet()

	// --- 3. Load the full package graph using the RuleSet ---
	slog.Debug("Loading full package graph...")
	walker := analyzer.NewPackageWalker()
	
	dependencyPaths := make([]string, 0)
	seenPaths := make(map[string]bool)
	seenPaths[initialPkg.PkgPath] = true // The initial package is always needed

	for sourcePkg, targetPkg := range ruleSet.PackagePairs {
		if !seenPaths[sourcePkg] {
			dependencyPaths = append(dependencyPaths, sourcePkg)
			seenPaths[sourcePkg] = true
		}
		if !seenPaths[targetPkg] {
			dependencyPaths = append(dependencyPaths, targetPkg)
			seenPaths[targetPkg] = true
		}
	}
	// Add package paths from TypePairs as dependencies
	for fqn := range ruleSet.TypePairs {
		pkgPath := fqn[:strings.LastIndex(fqn, ".")]
		if !seenPaths[pkgPath] {
			dependencyPaths = append(dependencyPaths, pkgPath)
			seenPaths[pkgPath] = true
		}
	}
    // Add package paths from PackageAliases as dependencies
    for _, path := range ruleSet.PackageAliases {
        if !seenPaths[path] {
            dependencyPaths = append(dependencyPaths, path)
            seenPaths[path] = true
        }
    }

	pkgs, err := walker.LoadFullGraph(initialPkg.PkgPath, dependencyPaths...)
	if err != nil {
		slog.Error("Failed to load full package graph", "error", err)
		os.Exit(1)
	}
	if packages.PrintErrors(pkgs) > 0 {
		slog.Error("Full package graph contains errors")
		os.Exit(1)
	}

	// --- 4. Generate Code ---
	slog.Debug("Generating code...")
	gen := generator.NewGenerator(walker, ruleSet)
	mainGeneratedCode, err := gen.Generate()
	if err != nil {
		slog.Error("Code generation failed", "error", err)
		os.Exit(1)
	}

	// --- 4. Write Main Output ---
	mainOutputFile := *output
	if mainOutputFile == "" {
		// Default to <package_name>.gen.go
		mainOutputFile = filepath.Join(sourceDir, strings.ToLower(initialPkg.Name)+".gen.go")
	}
	slog.Info("Writing main generated code", "file", mainOutputFile)
	err = os.WriteFile(mainOutputFile, mainGeneratedCode, 0644)
	if err != nil {
		slog.Error("Failed to write main output file", "error", err)
		os.Exit(1)
	}

	// --- 5. Write Custom Stubs Output (if any) ---
	customGeneratedCode := gen.CustomStubs()
	if len(customGeneratedCode) > 0 {
		customOutputFile := *customOutput
		if !filepath.IsAbs(customOutputFile) {
			customOutputFile = filepath.Join(sourceDir, customOutputFile)
		}
		slog.Info("Writing custom conversion stubs", "file", customOutputFile)
		err = os.WriteFile(customOutputFile, customGeneratedCode, 0644)
		if err != nil {
			slog.Error("Failed to write custom stubs file", "error", err)
			os.Exit(1)
		}
	}

	slog.Info("abgen finished successfully.")
}

func buildVersion(version, commit, date, builtBy, treeState string) goversion.Info {
	return goversion.GetVersionInfo(
		goversion.WithAppDetails(config.Application, config.Description, config.WebSite),
		func(i *goversion.Info) {
			i.ASCIIName = config.UI
			if commit != "" {
				i.GitCommit = commit
			}
			if version != "" {
				i.GitVersion = version
			}
			if treeState != "" {
				i.GitTreeState = treeState
			}
			if date != "" {
				i.BuildDate = date
			}
			if builtBy != "" {
				i.BuiltBy = builtBy
			}
		},
	)
}
