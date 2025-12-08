package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	goversion "github.com/caarlos0/go-version"

	"github.com/origadmin/abgen/internal/generator"
	"github.com/origadmin/abgen/internal/types"
)

var (
	version   = ""
	commit    = ""
	treeState = ""
	date      = ""
	builtBy   = ""
	debug     = false
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug, // Set log level to Debug
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == "source" {
				tmp := strings.Split(strings.Trim(a.Value.String(), "{}"), "/")
				a.Value = slog.StringValue(tmp[len(tmp)-1])
			}
			return a
		},
	})))
	//os.Chdir("D:\\workspace\\project\\golang\\origadmin\\backend")
	// 添加一个新的命令行参数
	templateDir := flag.String("templates", "", "自定义类型转换模板目录")
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		v := buildVersion(version, commit, date, builtBy, treeState)
		fmt.Println(v.String())
		fmt.Println("Usage: abgen <directory>")
		return
	}

	// 在创建生成器后设置模板目录
	generator := generator.NewGenerator()
	if *templateDir != "" {
		generator.SetTemplateDir(*templateDir)
	}

	// Determine the output directory based on the input argument
	inputPath := args[0]
	fileInfo, err := os.Stat(inputPath)
	if err != nil {
		slog.Error("获取输入路径信息失败", "错误", err)
		return
	}

	if fileInfo.IsDir() {
		generator.Output = inputPath
	} else {
		generator.Output = filepath.Dir(inputPath)
	}
	// The ParseSource also needs the correct input path, which is args[0]
	if err := generator.ParseSource(args[0]); err != nil {
		slog.Error("解析错误", "错误", err)
		return
	}

	if err := generator.Generate(); err != nil {
		slog.Error("生成错误", "错误", err)
	}
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
