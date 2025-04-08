package dirlisting

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"dumpall-go/internal/dumper"

	"github.com/PuerkitoBio/goquery"
)

// DirListingDumper 实现目录列表下载
type DirListingDumper struct {
	dumper.BaseDumper
}

// NewDirListingDumper 创建 DirListingDumper 实例
func NewDirListingDumper() *DirListingDumper {
	return &DirListingDumper{
		BaseDumper: dumper.BaseDumper{
			Name:        "dirlisting",
			Description: "下载目录列表中的文件",
		},
	}
}

// FileInfo 表示文件信息
type FileInfo struct {
	Name    string // 文件名
	URL     string // 文件URL
	Size    string // 文件大小
	ModTime string // 修改时间
}

// Check 检查目标是否存在目录列表
func (d *DirListingDumper) Check(targetURL string, client *http.Client) (bool, error) {
	// 确保URL以/结尾
	if !strings.HasSuffix(targetURL, "/") {
		targetURL += "/"
	}

	// 获取页面内容
	resp, err := client.Get(targetURL)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	// 解析HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return false, nil
	}

	// 检查是否包含目录列表特征
	links := doc.Find("a")
	if links.Length() == 0 {
		return false, nil
	}

	hasParentDir := false
	hasFiles := false

	links.Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		if href == "../" || href == ".." {
			hasParentDir = true
		} else if !strings.HasPrefix(href, "?") && !strings.HasPrefix(href, "#") {
			hasFiles = true
		}
	})

	return hasParentDir && hasFiles, nil
}

// Execute 执行下载操作
func (d *DirListingDumper) Execute(targetURL string, outdir string, proxy string, force bool, debug bool, workers int, progressCb dumper.ProgressCallback) error {
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

	// 获取页面内容
	resp, err := client.Get(targetURL)
	if err != nil {
		return fmt.Errorf("获取页面失败: %v", err)
	}
	defer resp.Body.Close()

	// 解析HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("解析HTML失败: %v", err)
	}

	// 下载所有文件
	links := doc.Find("a")
	links.Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		// 跳过父目录和特殊链接
		if href == "../" || href == ".." || strings.HasPrefix(href, "?") || strings.HasPrefix(href, "#") {
			return
		}

		// 构建完整URL
		baseURL, err := url.Parse(targetURL)
		if err != nil {
			if progressCb != nil {
				progressCb(href, 0, "URL解析失败")
			}
			return
		}

		fileURL, err := baseURL.Parse(href)
		if err != nil {
			if progressCb != nil {
				progressCb(href, 0, "URL解析失败")
			}
			return
		}

		// 构建本地路径
		localPath := filepath.Join(outdir, href)

		// 创建目录
		if strings.HasSuffix(href, "/") {
			if err := os.MkdirAll(localPath, 0755); err != nil {
				if progressCb != nil {
					progressCb(fileURL.String(), 0, "创建目录失败")
				}
				return
			}
			// 递归下载子目录
			if err := d.Execute(fileURL.String(), localPath, proxy, force, debug, workers, progressCb); err != nil {
				if progressCb != nil {
					progressCb(fileURL.String(), 0, "下载子目录失败")
				}
			}
			return
		}

		// 下载文件
		resp, err := client.Get(fileURL.String())
		if err != nil {
			if progressCb != nil {
				progressCb(fileURL.String(), 0, "下载失败")
			}
			return
		}

		// 调用进度回调
		if progressCb != nil {
			progressCb(fileURL.String(), resp.StatusCode, localPath)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return
		}

		// 创建本地文件
		f, err := os.Create(localPath)
		if err != nil {
			resp.Body.Close()
			if progressCb != nil {
				progressCb(fileURL.String(), 0, "创建文件失败")
			}
			return
		}

		// 写入文件内容
		_, err = io.Copy(f, resp.Body)
		resp.Body.Close()
		f.Close()

		if err != nil {
			if progressCb != nil {
				progressCb(fileURL.String(), 0, "写入失败")
			}
			return
		}
	})

	return nil
}

// Validate 验证URL是否有效
func (d *DirListingDumper) Validate(url string) error {
	return nil
}
