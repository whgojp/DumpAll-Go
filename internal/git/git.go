package git

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"dumpall-go/internal/dumper"
	"dumpall-go/pkg/utils"

	"github.com/fatih/color"
)

// GitDumper 实现 .git 源代码下载
type GitDumper struct {
	dumper.BaseDumper
}

// NewGitDumper 创建 GitDumper 实例
func NewGitDumper() *GitDumper {
	return &GitDumper{
		BaseDumper: dumper.BaseDumper{
			Name:        "git",
			Description: "下载 .git 源代码",
		},
	}
}

// Validate 验证URL是否有效
func (d *GitDumper) Validate(url string) error {
	if !strings.HasSuffix(url, ".git") && !strings.HasSuffix(url, ".git/") {
		return fmt.Errorf("URL必须以.git结尾")
	}
	return nil
}

// Dump 下载 Git 源代码（调用 Execute 实现，保留向后兼容）
func (g *GitDumper) Dump(targetURL, outdir, proxyAddr string, force bool) error {
	return g.Execute(targetURL, outdir, proxyAddr, force, false, 1, nil)
}

// Check 检查目标是否存在 .git 信息泄露
func (d *GitDumper) Check(targetURL string, client *http.Client) (bool, error) {
	// 确保URL以/结尾
	if !strings.HasSuffix(targetURL, "/") {
		targetURL += "/"
	}

	// 检查 .git/HEAD 文件
	headURL := targetURL + ".git/HEAD"
	resp, err := client.Head(headURL)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	return false, nil
}

// Execute 执行下载操作
func (d *GitDumper) Execute(targetURL string, outdir string, proxyAddr string, force bool, debug bool, workers int, progressCb dumper.ProgressCallback) error {
	// 创建HTTP客户端（支持 http/https/socks5 代理）
	color.Cyan("[Git] 初始化扫描: %s", targetURL)
	client, err := utils.CreateHTTPClient(proxyAddr)
	if err != nil {
		return fmt.Errorf("[Git] 创建HTTP客户端失败: %v", err)
	}

	// 确保URL以/结尾
	if !strings.HasSuffix(targetURL, "/") {
		targetURL += "/"
	}

	// 创建输出目录
	if err := os.MkdirAll(outdir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %v", err)
	}

	// 定义常见的Git文件
	gitFiles := []string{
		".git/HEAD",
		".git/config",
		".git/index",
		".git/description",
		".git/hooks/applypatch-msg.sample",
		".git/hooks/commit-msg.sample",
		".git/hooks/post-update.sample",
		".git/hooks/pre-applypatch.sample",
		".git/hooks/pre-commit.sample",
		".git/hooks/pre-push.sample",
		".git/hooks/pre-rebase.sample",
		".git/hooks/prepare-commit-msg.sample",
		".git/hooks/update.sample",
		".git/info/exclude",
	}

	color.Cyan("[Git] 共需检测 %d 个文件", len(gitFiles))
	// 下载文件
	for _, file := range gitFiles {
		fileURL := targetURL + file
		localPath := filepath.Join(outdir, file)

		// 创建目录
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			color.Red("[Git] 创建目录失败 %s: %v", filepath.Dir(localPath), err)
			if progressCb != nil {
				progressCb(fileURL, 0, "创建目录失败")
			}
			continue
		}

		// 下载文件
		color.Yellow("[Git] 请求: %s", fileURL)
		resp, err := client.Get(fileURL)
		if err != nil {
			color.Red("[Git] 请求失败: %s -> %v", fileURL, err)
			if progressCb != nil {
				progressCb(fileURL, 0, "下载失败")
			}
			continue
		}

		// 调用进度回调
		if progressCb != nil {
			progressCb(fileURL, resp.StatusCode, localPath)
		}

		if resp.StatusCode != http.StatusOK {
			color.Yellow("[Git] 跳过(状态码 %d): %s", resp.StatusCode, fileURL)
			resp.Body.Close()
			continue
		}

		// 创建本地文件
		f, err := os.Create(localPath)
		if err != nil {
			color.Red("[Git] 创建本地文件失败 %s: %v", localPath, err)
			resp.Body.Close()
			continue
		}

		// 写入文件内容
		_, err = io.Copy(f, resp.Body)
		resp.Body.Close()
		f.Close()

		if err != nil {
			color.Red("[Git] 写入失败 %s: %v", localPath, err)
			if progressCb != nil {
				progressCb(fileURL, 0, "写入失败")
			}
			continue
		}
		color.Green("[Git] 已保存: %s -> %s", fileURL, localPath)
	}

	return nil
}
