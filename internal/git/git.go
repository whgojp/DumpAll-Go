package git

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"dumpall-go/internal/dumper"
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

// 常见的 Git 文件
var gitFiles = []string{
	"HEAD",
	"config",
	"description",
	"index",
	"packed-refs",
	"refs/heads/master",
	"refs/heads/main",
	"refs/remotes/origin/HEAD",
	"refs/remotes/origin/master",
	"refs/remotes/origin/main",
	"refs/stash",
	"logs/HEAD",
	"logs/refs/heads/master",
	"logs/refs/heads/main",
	"logs/refs/remotes/origin/HEAD",
	"logs/refs/remotes/origin/master",
	"logs/refs/remotes/origin/main",
}

// Validate 验证URL是否有效
func (d *GitDumper) Validate(url string) error {
	if !strings.HasSuffix(url, ".git") && !strings.HasSuffix(url, ".git/") {
		return fmt.Errorf("URL必须以.git结尾")
	}
	return nil
}

// Dump 下载 Git 源代码
func (g *GitDumper) Dump(targetURL, outdir, proxy string, force bool) error {
	// 解析代理URL
	var httpClient *http.Client
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return fmt.Errorf("解析代理URL失败: %v", err)
		}
		httpClient = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			},
		}
	} else {
		httpClient = &http.Client{}
	}

	// 确保目标URL以/结尾
	if targetURL[len(targetURL)-1] != '/' {
		targetURL += "/"
	}

	// 下载常见的Git文件
	for _, file := range gitFiles {
		fileURL := targetURL + file
		outPath := filepath.Join(outdir, file)

		// 创建目录
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return fmt.Errorf("创建目录失败: %v", err)
		}

		// 下载文件
		resp, err := httpClient.Get(fileURL)
		if err != nil {
			continue // 忽略下载失败的文件
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue // 忽略不存在的文件
		}

		// 创建文件
		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("创建文件失败: %v", err)
		}
		defer f.Close()

		// 写入文件
		if _, err := io.Copy(f, resp.Body); err != nil {
			return fmt.Errorf("写入文件失败: %v", err)
		}
	}

	return nil
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
func (d *GitDumper) Execute(targetURL string, outdir string, proxy string, force bool, debug bool, workers int, progressCb dumper.ProgressCallback) error {
	// 创建HTTP客户端
	client := &http.Client{}
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return fmt.Errorf("代理设置错误: %v", err)
		}
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
		client.Transport = transport
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

	// 下载文件
	for _, file := range gitFiles {
		fileURL := targetURL + file
		localPath := filepath.Join(outdir, file)

		// 创建目录
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			if progressCb != nil {
				progressCb(fileURL, 0, "创建目录失败")
			}
			continue
		}

		// 下载文件
		resp, err := client.Get(fileURL)
		if err != nil {
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
			resp.Body.Close()
			continue
		}

		// 创建本地文件
		f, err := os.Create(localPath)
		if err != nil {
			resp.Body.Close()
			continue
		}

		// 写入文件内容
		_, err = io.Copy(f, resp.Body)
		resp.Body.Close()
		f.Close()

		if err != nil {
			if progressCb != nil {
				progressCb(fileURL, 0, "写入失败")
			}
			continue
		}
	}

	return nil
}
