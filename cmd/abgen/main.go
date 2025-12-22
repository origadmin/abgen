package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	goversion "github.com/caarlos0/go-version"

	"github.com/origadmin/abgen/internal/analyzer"
	"github.com/origadmin/abgen/internal/config"
	"github.com/origadmin/abgen/internal/generator"
)

var (
	version      = "0.0.1"
	commit       = ""
	treeState    = ""
	date         = ""
	builtBy      = ""
	debug        = flag.Bool("debug", false, "Enable debug logging")
	output       = flag.String("output", "", "Output file name for the main generated code. Defaults to <package_name>.gen.go.")
	customOutput = flag.String("custom-output", "custom.gen.go", "Output file name for custom conversion stubs.")
	logFile      = flag.String("log-file", "", "Path to a file where logs should be written. If empty, logs go to stderr.")
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

	logLevel := slog.LevelWarn
	if *debug {
		logLevel = slog.LevelDebug
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{
		Level: logLevel,
	})))

	v := buildVersion(version, commit, date, builtBy, treeState)
	if len(flag.Args()) == 0 {
		fmt.Println(v.String())
		fmt.Println("Usage: abgen [options] <source_directory>")
		flag.PrintDefaults()
		return
	}

	sourceDir := flag.Arg(0)
	slog.Info("Starting abgen", "sourceDir", sourceDir)

	// --- 1. Load Configuration ---
	slog.Debug("Parsing configuration...")
	parser := config.NewParser()
	cfg, err := parser.Parse(sourceDir)
	if err != nil {
		slog.Error("Failed to parse configuration", "error", err)
		os.Exit(1)
	}
	cfg.Version = version

	// Resolve output file paths and store them in GenerationContext
	mainOutputFile := *output
	if mainOutputFile == "" {
		mainOutputFile = filepath.Join(sourceDir, strings.ToLower(cfg.GenerationContext.PackageName)+".gen.go")
	} else if !filepath.IsAbs(mainOutputFile) {
		mainOutputFile = filepath.Join(sourceDir, mainOutputFile)
	}
	cfg.GenerationContext.MainOutputFile = mainOutputFile

	customOutputFile := *customOutput
	if !filepath.IsAbs(customOutputFile) {
		customOutputFile = filepath.Join(sourceDir, customOutputFile)
	}
	cfg.GenerationContext.CustomOutputFile = customOutputFile

	// --- 2. Analyze Types, Functions, and Aliases ---
	slog.Debug("Analyzing source code...")
	typeAnalyzer := analyzer.NewTypeAnalyzer()
	analysisResult, err := typeAnalyzer.Analyze(cfg)
	if err != nil {
		slog.Error("Failed to analyze types", "error", err)
		os.Exit(1)
	}

	// --- 3. Generate Code ---
	slog.Debug("Generating code...")
	gen := generator.NewCodeGenerator()
	response, err := gen.Generate(cfg, analysisResult)
	if err != nil {
		slog.Error("Code generation failed", "error", err)
		os.Exit(1)
	}

	// --- 4. Write Main Output ---
	slog.Info("Writing main generated code", "file", cfg.GenerationContext.MainOutputFile)
	err = os.WriteFile(cfg.GenerationContext.MainOutputFile, response.GeneratedCode, 0644)
	if err != nil {
		slog.Error("Failed to write main output file", "error", err)
		os.Exit(1)
	}

	// --- 5. Write Custom Stubs Output (if any) ---
	if len(response.CustomStubs) > 0 {
		slog.Info("Writing custom conversion stubs", "file", cfg.GenerationContext.CustomOutputFile)
		err = os.WriteFile(cfg.GenerationContext.CustomOutputFile, response.CustomStubs, 0644)
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
