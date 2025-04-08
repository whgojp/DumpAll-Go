package dsstore

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"dumpall-go/internal/dumper"
)

// DsStoreDumper 实现 DS_Store 子命令
type DsStoreDumper struct {
	dumper.BaseDumper
}

// NewDsStoreDumper 创建新的 DsStoreDumper 实例
func NewDsStoreDumper() *DsStoreDumper {
	return &DsStoreDumper{
		BaseDumper: dumper.BaseDumper{
			Name:        "dsstore",
			Description: "下载 .DS_Store 文件",
		},
	}
}

// DSStore 结构体用于解析 .DS_Store 文件
type DSStore struct {
	Magic   [4]byte  // 魔数，应该是 "Bud1"
	Version uint32   // 版本号
	Records []Record // 记录列表
}

// Record 表示一条记录
type Record struct {
	Name     string // 文件或目录名
	Type     string // 类型（文件或目录）
	Size     int64  // 大小
	Modified int64  // 修改时间
}

// Check 检查目标是否存在 .DS_Store 信息泄露
func (d *DsStoreDumper) Check(targetURL string, client *http.Client) (bool, error) {
	// 确保URL以/结尾
	if !strings.HasSuffix(targetURL, "/") {
		targetURL += "/"
	}

	// 检查 .DS_Store 文件
	dsStoreURL := targetURL + ".DS_Store"
	resp, err := client.Head(dsStoreURL)
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
func (d *DsStoreDumper) Execute(targetURL string, outdir string, proxy string, force bool, debug bool, workers int, progressCb dumper.ProgressCallback) error {
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

	// 下载 .DS_Store 文件
	fileURL := targetURL + ".DS_Store"
	localPath := filepath.Join(outdir, ".DS_Store")

	// 下载文件
	resp, err := client.Get(fileURL)
	if err != nil {
		if progressCb != nil {
			progressCb(fileURL, 0, "下载失败")
		}
		return err
	}
	defer resp.Body.Close()

	// 调用进度回调
	if progressCb != nil {
		progressCb(fileURL, resp.StatusCode, localPath)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("文件不存在")
	}

	// 创建本地文件
	f, err := os.Create(localPath)
	if err != nil {
		if progressCb != nil {
			progressCb(fileURL, 0, "创建文件失败")
		}
		return err
	}
	defer f.Close()

	// 写入文件内容
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		if progressCb != nil {
			progressCb(fileURL, 0, "写入失败")
		}
		return err
	}

	return nil
}

// Validate 验证目标URL是否有效
func (d *DsStoreDumper) Validate(url string) error {
	if !strings.HasSuffix(url, ".DS_Store") {
		return fmt.Errorf("URL必须以.DS_Store结尾")
	}
	return nil
}

// Dump 下载并解析 .DS_Store 文件
func (d *DsStoreDumper) Dump(targetURL, outdir, proxy string, force bool) error {
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

	// 下载 .DS_Store 文件
	resp, err := httpClient.Get(targetURL)
	if err != nil {
		return fmt.Errorf("下载文件失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("文件不存在: %s", targetURL)
	}

	// 读取文件内容
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取文件失败: %v", err)
	}

	// 保存原始文件
	outPath := filepath.Join(outdir, ".DS_Store")
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	if err := os.WriteFile(outPath, data, 0644); err != nil {
		return fmt.Errorf("保存文件失败: %v", err)
	}

	// 解析文件内容
	ds := &DSStore{}
	reader := bytes.NewReader(data)

	// 读取魔数
	if err := binary.Read(reader, binary.BigEndian, &ds.Magic); err != nil {
		return fmt.Errorf("读取魔数失败: %v", err)
	}

	// 验证魔数
	if string(ds.Magic[:]) != "Bud1" {
		return fmt.Errorf("无效的 .DS_Store 文件")
	}

	// 读取版本号
	if err := binary.Read(reader, binary.BigEndian, &ds.Version); err != nil {
		return fmt.Errorf("读取版本号失败: %v", err)
	}

	// TODO: 解析记录列表
	// .DS_Store 文件格式比较复杂，需要实现完整的解析逻辑

	return nil
}
