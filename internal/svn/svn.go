package svn

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"dumpall-go/internal/dumper"
	"dumpall-go/pkg/utils"

	"github.com/fatih/color"
	_ "modernc.org/sqlite"
)

// SvnDumper 实现 .svn 源代码下载
type SvnDumper struct {
	dumper.BaseDumper
}

// NewSvnDumper 创建 SvnDumper 实例
func NewSvnDumper() *SvnDumper {
	return &SvnDumper{
		BaseDumper: dumper.BaseDumper{
			Name:        "svn",
			Description: "下载 .svn 源代码",
		},
	}
}

// Check 检查目标是否存在 .svn 信息泄露
func (d *SvnDumper) Check(targetURL string, client *http.Client) (bool, error) {
	// 确保URL以/结尾
	if !strings.HasSuffix(targetURL, "/") {
		targetURL += "/"
	}

	// 检查 .svn/entries 文件
	entriesURL := targetURL + ".svn/entries"
	resp, err := client.Head(entriesURL)
	if err == nil {
		ok := resp.StatusCode == http.StatusOK
		resp.Body.Close()
		if ok {
			return true, nil
		}
	}

	// 检查 .svn/wc.db 文件 (SVN 1.7+)
	wcdbURL := targetURL + ".svn/wc.db"
	resp, err = client.Head(wcdbURL)
	if err == nil {
		ok := resp.StatusCode == http.StatusOK
		resp.Body.Close()
		if ok {
			return true, nil
		}
	}

	return false, nil
}

// Execute 执行下载操作
func (d *SvnDumper) Execute(targetURL string, outdir string, proxyAddr string, force bool, debug bool, workers int, progressCb dumper.ProgressCallback) error {
	// 创建HTTP客户端（支持 http/https/socks5 代理）
	color.Cyan("[SVN] 初始化扫描: %s", targetURL)
	client, err := utils.CreateHTTPClient(proxyAddr)
	if err != nil {
		return fmt.Errorf("[SVN] 创建HTTP客户端失败: %v", err)
	}

	// 确保URL以/结尾
	if !strings.HasSuffix(targetURL, "/") {
		targetURL += "/"
	}

	// 创建输出目录
	if err := os.MkdirAll(outdir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %v", err)
	}

	// 定义常见的SVN文件
	svnFiles := []string{
		".svn/entries",
		".svn/wc.db",
		".svn/format",
		".svn/all-wcprops",
		".svn/props",
		".svn/text-base",
		".svn/tmp",
	}

	color.Cyan("[SVN] 共需检测 %d 个文件", len(svnFiles))
	// 下载文件
	for _, file := range svnFiles {
		fileURL := targetURL + file
		localPath := filepath.Join(outdir, file)

		// 创建目录
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			color.Red("[SVN] 创建目录失败 %s: %v", filepath.Dir(localPath), err)
			if progressCb != nil {
				progressCb(fileURL, 0, "创建目录失败")
			}
			continue
		}

		// 下载文件
		color.Yellow("[SVN] 请求: %s", fileURL)
		resp, err := client.Get(fileURL)
		if err != nil {
			color.Red("[SVN] 请求失败: %s -> %v", fileURL, err)
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
			color.Yellow("[SVN] 跳过(状态码 %d): %s", resp.StatusCode, fileURL)
			resp.Body.Close()
			continue
		}

		// 创建本地文件
		f, err := os.Create(localPath)
		if err != nil {
			color.Red("[SVN] 创建本地文件失败 %s: %v", localPath, err)
			resp.Body.Close()
			continue
		}

		// 写入文件内容
		_, err = io.Copy(f, resp.Body)
		resp.Body.Close()
		f.Close()

		if err != nil {
			color.Red("[SVN] 写入失败 %s: %v", localPath, err)
			if progressCb != nil {
				progressCb(fileURL, 0, "写入失败")
			}
			continue
		}
		color.Green("[SVN] 已保存: %s -> %s", fileURL, localPath)
	}

	return nil
}

// Validate 验证URL是否有效
func (d *SvnDumper) Validate(url string) error {
	if !strings.HasSuffix(url, ".svn") && !strings.HasSuffix(url, ".svn/") {
		return fmt.Errorf("URL必须以.svn结尾")
	}
	return nil
}

// --------------------------------------------------------------------------
// SVN Extract：从 .svn 信息泄露中还原完整源代码
// --------------------------------------------------------------------------

// SvnFileEntry 表示从 SVN 元数据中解析出的一条文件记录
type SvnFileEntry struct {
	RelPath string // 相对于项目根的路径，例如 "src/main.go"
	Kind    string // "file" 或 "dir"
}

// ParseEntriesFromData 解析 SVN 1.6 及更早的 entries 文件（纯文本格式）
//
// SVN entries 真实格式（version 8~10）：
//
//	第一行：格式版本号（如 "10"），属于第0块
//	之后每条记录以 \f（换页符, 0x0C）分隔
//	每条记录字段以 \n 分隔，顺序为：
//	  [0] name      → 文件/目录名，根目录节点此行为空字符串
//	  [1] kind      → "file" 或 "dir"
//	  [2] revision
//	  [3] url       → 仅根目录节点有值
//	  ...
//
// 示例（真实数据）：
//
//	10\n\fdir\n354\nhttps://...\n...\f\napi.php\nfile\n...\f\nindex.php\nfile\n...
//
// 注意：根节点 name 为空（\f 紧跟 \n），直接跳过。
func ParseEntriesFromData(data []byte) []SvnFileEntry {
	var entries []SvnFileEntry

	// 以换页符 \f 切割记录
	// records[0] 为版本号行（如 "10\n"），后续每块对应一条 entry
	records := strings.Split(string(data), "\f")

	for i, record := range records {
		// 第0块仅含版本号，跳过
		if i == 0 {
			continue
		}

		// 按 \n 切割字段：
		//   [0] name  — 文件/目录名；根目录节点此行为空字符串
		//   [1] kind  — "file" 或 "dir"
		// 注意：不裁剪末尾空行，因为 lines[0] 本身可能就是空字符串（根目录），
		// 裁剪末尾会把有效的空 name 误删，导致 lines[1] 越界。
		lines := strings.Split(record, "\n")
		if len(lines) < 2 {
			continue
		}

		name := lines[0]
		kind := lines[1]

		// name 为空 = 根目录节点，跳过
		if name == "" {
			continue
		}
		if kind == "file" || kind == "dir" {
			entries = append(entries, SvnFileEntry{RelPath: name, Kind: kind})
		}
	}

	return entries
}

// ParseWcDbFromFile 解析 SVN 1.7+ 的 wc.db（SQLite）文件，返回被追踪的文件列表
//
// SVN wc.db schema（libsvn_wc/wc-metadata.sql）中 NODES.kind 为 TEXT 字段，
// 取值为 'file'、'dir'、'symlink'、'subdir'、'unknown'，不是整数。
func ParseWcDbFromFile(dbPath string) ([]SvnFileEntry, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开 wc.db 失败: %v", err)
	}
	defer db.Close()

	// local_relpath 为空字符串代表仓库根节点，排除掉
	rows, err := db.Query(`
		SELECT local_relpath, kind
		FROM NODES
		WHERE local_relpath != ''
		ORDER BY local_relpath
	`)
	if err != nil {
		return nil, fmt.Errorf("查询 wc.db NODES 表失败: %v", err)
	}
	defer rows.Close()

	var entries []SvnFileEntry
	for rows.Next() {
		var relPath, kind string
		if err := rows.Scan(&relPath, &kind); err != nil {
			continue
		}
		// 只保留 file 和 dir，忽略 symlink/subdir/unknown
		if kind == "file" || kind == "dir" {
			entries = append(entries, SvnFileEntry{RelPath: relPath, Kind: kind})
		}
	}
	return entries, rows.Err()
}

// downloadFile 下载单个远程文件到本地路径
func downloadFile(client *http.Client, fileURL, localPath string) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	resp, err := client.Get(fileURL)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("写入失败: %v", err)
	}
	return nil
}

// extractEntriesRecursive 递归处理一个目录的 entries 文件，下载所有文件并继续递归子目录
// 参考 dvcs-ripper 的标准实现：每个子目录都有自己的 .svn/entries，需要分别请求解析。
//
// 参数：
//
//	client      — HTTP 客户端
//	targetURL   — 当前目录的 Web 根 URL（末尾含 /），例如 http://example.com/src/
//	relBase     — 当前目录相对于项目根的路径前缀，例如 "src/"（根目录为 ""）
//	extractDir  — 本地输出根目录
//	visited     — 已访问的目录集合（防止循环）
//	counters    — [0]=success, [1]=fail
//	progressCb  — 进度回调
func extractEntriesRecursive(
	client *http.Client,
	targetURL string,
	relBase string,
	extractDir string,
	visited map[string]bool,
	counters *[2]int,
	progressCb dumper.ProgressCallback,
) {
	if visited[targetURL] {
		return
	}
	visited[targetURL] = true

	svnBase := targetURL + ".svn/"
	entriesURL := svnBase + "entries"
	color.Yellow("[SVN-Extract] 解析 entries: %s", entriesURL)

	// 下载 entries 文件到临时内存
	resp, err := client.Get(entriesURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		color.Yellow("[SVN-Extract] 无法访问 entries: %s", entriesURL)
		return
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil || len(data) == 0 {
		return
	}

	entries := ParseEntriesFromData(data)
	color.Cyan("[SVN-Extract] %s → 解析出 %d 个条目", entriesURL, len(entries))

	for _, entry := range entries {
		// 当前文件/目录相对于项目根的完整路径
		fullRelPath := relBase + entry.RelPath

		if entry.Kind == "dir" {
			// 创建本地目录
			localDir := filepath.Join(extractDir, filepath.FromSlash(fullRelPath))
			if err := os.MkdirAll(localDir, 0755); err != nil {
				color.Red("[SVN-Extract] 创建目录失败 %s: %v", fullRelPath, err)
				continue
			}
			color.Cyan("[SVN-Extract] 目录: %s", fullRelPath)
			// 递归进入子目录（参考 dvcs-ripper 的核心做法）
			subURL := targetURL + entry.RelPath + "/"
			extractEntriesRecursive(client, subURL, fullRelPath+"/", extractDir, visited, counters, progressCb)
			continue
		}

		// 文件：尝试两种下载策略
		localPath := filepath.Join(extractDir, filepath.FromSlash(fullRelPath))

		// 策略1：从 .svn/text-base/<name>.svn-base 下载
		// SVN 1.6 在每个目录的 .svn/text-base/ 下存放该目录内文件的原始副本
		textBaseURL := svnBase + "text-base/" + entry.RelPath + ".svn-base"
		color.Yellow("[SVN-Extract] 尝试 text-base: %s", textBaseURL)
		if err := downloadFile(client, textBaseURL, localPath); err == nil {
			color.Green("[SVN-Extract] ✓ text-base: %s", fullRelPath)
			if progressCb != nil {
				progressCb(textBaseURL, http.StatusOK, localPath)
			}
			counters[0]++
			continue
		}

		// 策略2：直接从 Web 路径下载（服务器目录遍历可访问时有效）
		directURL := targetURL + entry.RelPath
		color.Yellow("[SVN-Extract] 尝试直接下载: %s", directURL)
		if err := downloadFile(client, directURL, localPath); err == nil {
			color.Green("[SVN-Extract] ✓ 直接下载: %s", fullRelPath)
			if progressCb != nil {
				progressCb(directURL, http.StatusOK, localPath)
			}
			counters[0]++
			continue
		}

		color.Red("[SVN-Extract] ✗ 失败: %s", fullRelPath)
		if progressCb != nil {
			progressCb(directURL, 0, "提取失败")
		}
		counters[1]++
	}
}

// Extract 从目标 URL 的 .svn 信息泄露中还原完整源代码
//
// 流程：
//  1. 优先尝试 entries（SVN 1.6-）递归提取模式
//     参照 dvcs-ripper 标准做法：根目录 entries → 子目录各自的 entries → 递归
//  2. 若 entries 不可用（SVN 1.7+），改用 wc.db（SQLite）一次性获取全部路径
//  3. 每个文件先尝试 .svn/text-base/*.svn-base，再尝试直接 HTTP 下载
//  4. 还原为真实源码目录结构输出到 outdir/extracted/
func (d *SvnDumper) Extract(targetURL string, outdir string, proxyAddr string, workers int, progressCb dumper.ProgressCallback) error {
	color.Cyan("[SVN-Extract] 开始提取源代码: %s", targetURL)

	client, err := utils.CreateHTTPClient(proxyAddr)
	if err != nil {
		return fmt.Errorf("[SVN-Extract] 创建HTTP客户端失败: %v", err)
	}

	if !strings.HasSuffix(targetURL, "/") {
		targetURL += "/"
	}

	extractDir := filepath.Join(outdir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %v", err)
	}

	// ── 阶段1：先探测是 entries（SVN 1.6-）还是 wc.db（SVN 1.7+）─────────
	svnBase := targetURL + ".svn/"
	entriesURL := svnBase + "entries"

	color.Yellow("[SVN-Extract] 探测 SVN 版本格式: %s", entriesURL)
	resp, err := client.Get(entriesURL)
	if err != nil {
		return fmt.Errorf("[SVN-Extract] 无法访问目标: %v", err)
	}
	entriesData, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	useEntries := false
	if resp.StatusCode == http.StatusOK && len(entriesData) > 3 {
		// 检查首行是否是纯数字版本号（8/9/10 表示旧格式；"12" 是 SVN 1.7 的 stub）
		firstLine := strings.SplitN(string(entriesData), "\n", 2)[0]
		firstLine = strings.TrimSpace(firstLine)
		if firstLine == "8" || firstLine == "9" || firstLine == "10" {
			useEntries = true
			color.Green("[SVN-Extract] 检测到 entries 格式 version=%s（SVN 1.6-），使用递归提取模式", firstLine)
		} else if firstLine == "12" {
			color.Yellow("[SVN-Extract] entries version=12（SVN 1.7+ stub），改用 wc.db 模式")
		} else {
			color.Yellow("[SVN-Extract] entries 首行 %q 无法识别，尝试 wc.db", firstLine)
		}
	}

	var counters [2]int // [0]=success [1]=fail

	if useEntries {
		// ── entries 递归模式（SVN 1.6-）────────────────────────────────────
		visited := make(map[string]bool)
		// 将已下载的 entries 内容直接解析，不重复请求
		entries := ParseEntriesFromData(entriesData)
		color.Cyan("[SVN-Extract] 根目录 entries 解析出 %d 个条目", len(entries))

		for _, entry := range entries {
			if entry.Kind == "dir" {
				localDir := filepath.Join(extractDir, filepath.FromSlash(entry.RelPath))
				if err := os.MkdirAll(localDir, 0755); err != nil {
					color.Red("[SVN-Extract] 创建目录失败 %s: %v", entry.RelPath, err)
					continue
				}
				color.Cyan("[SVN-Extract] 目录: %s", entry.RelPath)
				subURL := targetURL + entry.RelPath + "/"
				extractEntriesRecursive(client, subURL, entry.RelPath+"/", extractDir, visited, &counters, progressCb)
				continue
			}

			localPath := filepath.Join(extractDir, filepath.FromSlash(entry.RelPath))
			textBaseURL := svnBase + "text-base/" + entry.RelPath + ".svn-base"
			color.Yellow("[SVN-Extract] 尝试 text-base: %s", textBaseURL)
			if err := downloadFile(client, textBaseURL, localPath); err == nil {
				color.Green("[SVN-Extract] ✓ text-base: %s", entry.RelPath)
				if progressCb != nil {
					progressCb(textBaseURL, http.StatusOK, localPath)
				}
				counters[0]++
				continue
			}
			directURL := targetURL + entry.RelPath
			color.Yellow("[SVN-Extract] 尝试直接下载: %s", directURL)
			if err := downloadFile(client, directURL, localPath); err == nil {
				color.Green("[SVN-Extract] ✓ 直接下载: %s", entry.RelPath)
				if progressCb != nil {
					progressCb(directURL, http.StatusOK, localPath)
				}
				counters[0]++
				continue
			}
			color.Red("[SVN-Extract] ✗ 失败: %s", entry.RelPath)
			if progressCb != nil {
				progressCb(directURL, 0, "提取失败")
			}
			counters[1]++
		}

	} else {
		// ── wc.db 模式（SVN 1.7+）──────────────────────────────────────────
		wcdbURL := svnBase + "wc.db"
		color.Yellow("[SVN-Extract] 尝试下载 wc.db: %s", wcdbURL)
		svnDir := filepath.Join(outdir, ".svn")
		if err := os.MkdirAll(svnDir, 0755); err != nil {
			return fmt.Errorf("创建 .svn 缓存目录失败: %v", err)
		}
		wcdbLocal := filepath.Join(svnDir, "wc.db")
		if err := downloadFile(client, wcdbURL, wcdbLocal); err != nil {
			return fmt.Errorf("[SVN-Extract] wc.db 不可用: %v，目标可能不存在 SVN 泄露", err)
		}
		fileEntries, parseErr := ParseWcDbFromFile(wcdbLocal)
		if parseErr != nil {
			return fmt.Errorf("[SVN-Extract] wc.db 解析失败: %v", parseErr)
		}
		color.Green("[SVN-Extract] wc.db 解析成功，发现 %d 个条目", len(fileEntries))

		for _, entry := range fileEntries {
			if entry.Kind == "dir" {
				localDir := filepath.Join(extractDir, filepath.FromSlash(entry.RelPath))
				if err := os.MkdirAll(localDir, 0755); err != nil {
					color.Red("[SVN-Extract] 创建目录失败 %s: %v", entry.RelPath, err)
				} else {
					color.Cyan("[SVN-Extract] 目录: %s", entry.RelPath)
				}
				continue
			}
			localPath := filepath.Join(extractDir, filepath.FromSlash(entry.RelPath))
			directURL := targetURL + entry.RelPath
			color.Yellow("[SVN-Extract] 尝试直接下载: %s", directURL)
			if err := downloadFile(client, directURL, localPath); err == nil {
				color.Green("[SVN-Extract] ✓ 直接下载: %s", entry.RelPath)
				if progressCb != nil {
					progressCb(directURL, http.StatusOK, localPath)
				}
				counters[0]++
			} else {
				color.Red("[SVN-Extract] ✗ 失败: %s", entry.RelPath)
				if progressCb != nil {
					progressCb(directURL, 0, "提取失败")
				}
				counters[1]++
			}
		}
	}

	color.Green("[SVN-Extract] 提取完成！成功: %d，失败: %d，输出目录: %s", counters[0], counters[1], extractDir)
	return nil
}
