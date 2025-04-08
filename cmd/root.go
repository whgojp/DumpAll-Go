package cmd

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"dumpall-go/internal/dirlisting"
	"dumpall-go/internal/dsstore"
	"dumpall-go/internal/git"
	"dumpall-go/internal/svn"
	"dumpall-go/pkg/utils"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	targetURL string
	urlFile   string
	outdir    string
	proxy     string
	workers   int
)

// 定义颜色输出
var (
	successColor = color.New(color.FgGreen)
	errorColor   = color.New(color.FgRed)
	infoColor    = color.New(color.FgCyan)
)

// progressCallback 处理下载进度
func progressCallback(url string, statusCode int, msg string) {
	if statusCode == http.StatusOK {
		fmt.Printf("[%d] %s\n", statusCode, url)
	}
}

// RootCmd 代表基础命令
var RootCmd = &cobra.Command{
	Use:   "dumpall-go",
	Short: "信息泄露和源代码泄露利用工具",
	Long: `DumpAll-Go 是一个用于自动化信息收集和敏感信息提取的工具。
支持以下场景：
  .git源代码泄漏
  .svn源代码泄漏
  .DS_Store信息泄漏
  目录列出信息泄漏`,
	Run: func(cmd *cobra.Command, args []string) {
		if targetURL == "" && urlFile == "" {
			errorColor.Println("错误: 必须指定目标URL或URL文件")
			cmd.Help()
			return
		}

		var urls []string
		var err error
		if urlFile != "" {
			urls, err = utils.ReadURLsFromFile(urlFile)
			if err != nil {
				errorColor.Printf("读取URL文件失败: %v\n", err)
				return
			}
		} else {
			urls = []string{targetURL}
		}

		if len(urls) == 0 {
			fmt.Println("未指定URL")
			return
		}

		for _, url := range urls {
			if err := utils.ValidateURL(url); err != nil {
				fmt.Printf("URL格式错误: %v\n", err)
				return
			}
		}

		if err := os.MkdirAll(outdir, 0755); err != nil {
			errorColor.Printf("创建输出目录失败: %v\n", err)
			return
		}

		var tasks []utils.Task
		for _, url := range urls {
			task := utils.Task{
				URL:    url,
				Outdir: filepath.Join(outdir, utils.GetHostname(url)),
				Proxy:  proxy,
				Type:   "all",
			}
			tasks = append(tasks, task)
		}

		results := utils.ProcessTasks(tasks, func(task utils.Task) utils.Result {
			result := utils.Result{
				URL:    task.URL,
				Start:  time.Now(),
				Output: task.Outdir,
			}

			gitDumper := git.NewGitDumper()
			svnDumper := svn.NewSvnDumper()
			dsstoreDumper := dsstore.NewDsStoreDumper()
			dirlistingDumper := dirlisting.NewDirListingDumper()

			err := gitDumper.Execute(task.URL, task.Outdir, task.Proxy, false, false, workers, progressCallback)
			if err != nil {
				result.Error = err
			}

			err = svnDumper.Execute(task.URL, task.Outdir, task.Proxy, false, false, workers, progressCallback)
			if err != nil {
				result.Error = err
			}

			err = dsstoreDumper.Execute(task.URL, task.Outdir, task.Proxy, false, false, workers, progressCallback)
			if err != nil {
				result.Error = err
			}

			err = dirlistingDumper.Execute(task.URL, task.Outdir, task.Proxy, false, false, workers, progressCallback)
			if err != nil {
				result.Error = err
			}

			result.End = time.Now()
			result.Success = result.Error == nil
			return result
		}, workers, utils.NewLogger(false))

		for _, result := range results {
			if result.Success {
				successColor.Printf("成功: %s -> %s\n", result.URL, result.Output)
			} else {
				errorColor.Printf("失败: %s -> %v\n", result.URL, result.Error)
			}
		}
	},
	DisableFlagsInUseLine: true,
	DisableAutoGenTag:     true,
}

func init() {
	RootCmd.Flags().SortFlags = false
	RootCmd.PersistentFlags().SortFlags = false
	RootCmd.LocalFlags().SortFlags = false
	RootCmd.InheritedFlags().SortFlags = false

	RootCmd.PersistentFlags().StringVarP(&targetURL, "url", "u", "", "目标URL")
	RootCmd.PersistentFlags().StringVarP(&urlFile, "file", "f", "", "包含URL列表的文件")
	RootCmd.PersistentFlags().StringVarP(&outdir, "outdir", "o", "output", "输出目录")
	RootCmd.PersistentFlags().StringVarP(&proxy, "proxy", "p", "", "代理服务器 (例如: http://127.0.0.1:8080)")
	RootCmd.PersistentFlags().IntVarP(&workers, "workers", "w", 10, "并发工作线程数")
}

// Execute 执行命令
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
