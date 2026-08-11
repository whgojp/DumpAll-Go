package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"dumpall-go/internal/svn"
	"dumpall-go/pkg/utils"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	svnExtractURL    string
	svnExtractOutdir string
	svnExtractProxy  string
	svnExtractWorkers int
)

// svnExtractCmd 实现 svn-extract 子命令
// 用于从已确认存在 .svn 信息泄露的目标中提取完整源代码
var svnExtractCmd = &cobra.Command{
	Use:   "svn-extract",
	Short: "从 .svn 信息泄露中提取完整源代码",
	Long: `svn-extract 专用于从已确认存在 .svn 信息泄露的目标中还原完整源代码。

支持两种 SVN 版本格式：
  SVN 1.6 及更早  使用 .svn/entries 文件（纯文本）列举被追踪文件，
                  并从 .svn/text-base/*.svn-base 获取文件内容。
  SVN 1.7 及以上  使用 .svn/wc.db（SQLite 数据库）列举被追踪文件，
                  并尝试直接从服务器路径下载文件内容。

提取策略（对每个文件按优先级尝试）：
  1. .svn/text-base/<file>.svn-base  — SVN 1.6 原始副本
  2. 直接 HTTP 请求目标路径          — 目录遍历/开放访问时有效

输出目录结构：
  <outdir>/
    .svn/          — 缓存的 SVN 元数据文件
    extracted/     — 还原的真实源码目录结构`,

	Example: `  # 对单个目标提取 SVN 源码
  dumpall-go svn-extract -u http://example.com/

  # 指定输出目录
  dumpall-go svn-extract -u http://example.com/ -o ./leaked-src

  # 通过 SOCKS5 代理提取
  dumpall-go svn-extract -u http://example.com/ -p socks5://127.0.0.1:1080`,

	DisableFlagsInUseLine: true,
	DisableAutoGenTag:     true,

	Run: func(cmd *cobra.Command, args []string) {
		if svnExtractURL == "" {
			color.Red("错误: 必须通过 -u/--url 指定目标 URL")
			cmd.Help()
			return
		}

		if err := utils.ValidateURL(svnExtractURL); err != nil {
			color.Red("URL 格式错误: %v", err)
			return
		}

		outdir := svnExtractOutdir
		if outdir == "" {
			outdir = filepath.Join("output", utils.GetHostname(svnExtractURL))
		}

		if err := os.MkdirAll(outdir, 0755); err != nil {
			color.Red("创建输出目录失败: %v", err)
			return
		}

		color.Cyan("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		color.Cyan("  SVN Extract — 源代码提取")
		color.Cyan("  目标: %s", svnExtractURL)
		color.Cyan("  输出: %s", outdir)
		if svnExtractProxy != "" {
			color.Cyan("  代理: %s", svnExtractProxy)
		}
		color.Cyan("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		dumper := svn.NewSvnDumper()
		err := dumper.Extract(
			svnExtractURL,
			outdir,
			svnExtractProxy,
			svnExtractWorkers,
			func(url string, statusCode int, msg string) {
				if statusCode == 200 {
					fmt.Printf("  [200] %s\n", url)
				}
			},
		)
		if err != nil {
			color.Red("\n提取失败: %v", err)
			return
		}

		extractedDir := filepath.Join(outdir, "extracted")
		color.Green("\n提取成功！源代码已还原至: %s", extractedDir)
	},
}

func init() {
	svnExtractCmd.Flags().SortFlags = false

	svnExtractCmd.Flags().StringVarP(&svnExtractURL, "url", "u", "", "目标 URL（必填，例如: http://example.com/）")
	svnExtractCmd.Flags().StringVarP(&svnExtractOutdir, "outdir", "o", "", "输出目录（默认: output/<hostname>）")
	svnExtractCmd.Flags().StringVarP(&svnExtractProxy, "proxy", "p", "", "代理服务器 (支持: http://host:port | socks5://host:port | socks5h://host:port)")
	svnExtractCmd.Flags().IntVarP(&svnExtractWorkers, "workers", "w", 10, "并发下载线程数")

	RootCmd.AddCommand(svnExtractCmd)
}
