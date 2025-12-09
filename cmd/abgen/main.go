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

	"github.com/origadmin/abgen/internal/ast"
	"github.com/origadmin/abgen/internal/generator"
	"github.com/origadmin/abgen/internal/types"
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

	// --- 1. Load the directive package ---
	slog.Debug("Loading directive package...")
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedImports | packages.NeedDeps | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  sourceDir,
		// Allow parsing packages with type errors since abgen may generate the missing types.
		// We will log these errors but not fail immediately.
		// AllowErrors: true, // Removed as it's not a valid field
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		slog.Error("Failed to load directive package", "error", err)
		os.Exit(1)
	}
	if packages.PrintErrors(pkgs) > 0 {
		slog.Warn("Type errors found in directive package (may be expected for types that abgen will generate)")
	}
	if len(pkgs) == 0 {
		slog.Error("No packages found in source directory", "sourceDir", sourceDir)
		os.Exit(1)
	}
	directivePkg := pkgs[0]

	// --- 2. Create and Run Walker ---
	slog.Debug("Initializing walker and analysis...")
	walker := ast.NewPackageWalker()
	if err := walker.Analyze(directivePkg); err != nil {
		slog.Error("Analysis failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Analysis complete.")

	// --- 3. Generate Code ---
	slog.Debug("Generating code...")
	gen := generator.NewGenerator(walker)
	mainGeneratedCode, err := gen.Generate()
	if err != nil {
		slog.Error("Code generation failed", "error", err)
		os.Exit(1)
	}

	// --- 4. Write Main Output ---
	mainOutputFile := *output
	if mainOutputFile == "" {
		// Default to <package_name>.gen.go
		mainOutputFile = filepath.Join(sourceDir, strings.ToLower(directivePkg.Name)+".gen.go")
	}
	slog.Info("Writing main generated code", "file", mainOutputFile)
	err = os.WriteFile(mainOutputFile, mainGeneratedCode, 0644)
	if err != nil {
		slog.Error("Failed to write main output file", "error", err)
		os.Exit(1)
	}

	// --- 5. Write Custom Stubs Output (if any) ---
	customGeneratedCode := gen.CustomStubs() // Assuming CustomStubs() method exists on Generator
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
		goversion.WithAppDetails(types.Application, types.Description, types.WebSite),
		func(i *goversion.Info) {
			i.ASCIIName = types.UI
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
