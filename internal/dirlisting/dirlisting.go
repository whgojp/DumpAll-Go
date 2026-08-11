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
	"dumpall-go/pkg/utils"

	"github.com/PuerkitoBio/goquery"
	"github.com/fatih/color"
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
func (d *DirListingDumper) Execute(targetURL string, outdir string, proxyAddr string, force bool, debug bool, workers int, progressCb dumper.ProgressCallback) error {
	// 创建HTTP客户端（支持 http/https/socks5 代理）
	color.Cyan("[DirListing] 初始化扫描: %s", targetURL)
	client, err := utils.CreateHTTPClient(proxyAddr)
	if err != nil {
		return fmt.Errorf("[DirListing] 创建HTTP客户端失败: %v", err)
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
	color.Yellow("[DirListing] 请求目录页面: %s", targetURL)
	resp, err := client.Get(targetURL)
	if err != nil {
		color.Red("[DirListing] 请求失败: %s -> %v", targetURL, err)
		return fmt.Errorf("获取页面失败: %v", err)
	}
	defer resp.Body.Close()
	color.Cyan("[DirListing] 响应状态码: %d  URL: %s", resp.StatusCode, targetURL)

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

		// href 可能是协议相对(//other.com/x)或绝对URL(http://other.com/x)，
		// 一旦跨 host/scheme，说明是外部链接（广告、CDN、别的站点），不应下载，
		// 否则会把 host 名（如 "xxx.com"）当成本地目录名拼进 relPath，
		// 导致后续 MkdirAll/Create 路径混乱甚至 "no such file or directory"。
		if !strings.EqualFold(fileURL.Host, baseURL.Host) {
			color.Yellow("[DirListing] 跳过跨域链接: %s", fileURL.String())
			return
		}

		// 构建本地路径：用 fileURL.Path 相对于 baseURL.Path 的部分，防止路径穿越
		// 例如 href="../../../etc/passwd" 经 url.Parse 解析后 fileURL 可能超出预期目录
		relPath := strings.TrimPrefix(fileURL.Path, baseURL.Path)
		relPath = filepath.FromSlash(relPath)
		// filepath.Join + Clean 会消除 .. 但不能阻止绝对路径；手动加前缀校验
		localPath := filepath.Join(outdir, relPath)
		// 路径穿越检查：确保 localPath 在 outdir 内
		cleanOut := filepath.Clean(outdir) + string(os.PathSeparator)
		if !strings.HasPrefix(filepath.Clean(localPath)+string(os.PathSeparator), cleanOut) {
			color.Red("[DirListing] 路径穿越，跳过: %s", fileURL.String())
			return
		}

		// 创建目录
		if strings.HasSuffix(href, "/") {
			if err := os.MkdirAll(localPath, 0755); err != nil {
				color.Red("[DirListing] 创建子目录失败 %s: %v", localPath, err)
				if progressCb != nil {
					progressCb(fileURL.String(), 0, "创建目录失败")
				}
				return
			}
			color.Cyan("[DirListing] 递归扫描子目录: %s", fileURL.String())
			// 递归下载子目录
			if err := d.Execute(fileURL.String(), localPath, proxyAddr, force, debug, workers, progressCb); err != nil {
				color.Red("[DirListing] 下载子目录失败 %s: %v", fileURL.String(), err)
				if progressCb != nil {
					progressCb(fileURL.String(), 0, "下载子目录失败")
				}
			}
			return
		}

		// 下载文件
		color.Yellow("[DirListing] 请求文件: %s", fileURL.String())
		resp, err := client.Get(fileURL.String())
		if err != nil {
			color.Red("[DirListing] 请求失败: %s -> %v", fileURL.String(), err)
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
			color.Yellow("[DirListing] 跳过(状态码 %d): %s", resp.StatusCode, fileURL.String())
			resp.Body.Close()
			return
		}

		// 创建父目录（文件可能位于深层相对路径下，父目录未必已存在）
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			color.Red("[DirListing] 创建父目录失败 %s: %v", filepath.Dir(localPath), err)
			resp.Body.Close()
			if progressCb != nil {
				progressCb(fileURL.String(), 0, "创建目录失败")
			}
			return
		}

		// 创建本地文件
		f, err := os.Create(localPath)
		if err != nil {
			color.Red("[DirListing] 创建本地文件失败 %s: %v", localPath, err)
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
			color.Red("[DirListing] 写入失败 %s: %v", localPath, err)
			if progressCb != nil {
				progressCb(fileURL.String(), 0, "写入失败")
			}
			return
		}
		color.Green("[DirListing] 已保存: %s -> %s", fileURL.String(), localPath)
	})

	return nil
}

// Validate 验证URL是否有效
func (d *DirListingDumper) Validate(url string) error {
	return nil
}
