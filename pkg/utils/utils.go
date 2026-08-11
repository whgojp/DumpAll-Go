package utils

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
)

type Task struct {
	URL    string
	Outdir string
	Proxy  string
	Type   string
}

type Result struct {
	URL     string
	Error   error
	Output  string
	Start   time.Time
	End     time.Time
	Success bool
}

type Logger struct {
	debug bool
}

func NewLogger(debug bool) *Logger {
	return &Logger{debug: debug}
}

func (l *Logger) Info(format string, args ...interface{}) {
	color.Cyan(format, args...)
}

func (l *Logger) Success(format string, args ...interface{}) {
	color.Green(format, args...)
}

func (l *Logger) Error(format string, args ...interface{}) {
	color.Red(format, args...)
}

func (l *Logger) Debug(format string, args ...interface{}) {
	if l.debug {
		color.Yellow(format, args...)
	}
}

func ReadURLsFromFile(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		url := strings.TrimSpace(scanner.Text())
		if url != "" {
			urls = append(urls, url)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件失败: %v", err)
	}

	fmt.Printf("总共读取到 %d 个URL\n", len(urls))
	return urls, nil
}

func ProcessTasks(tasks []Task, workerFunc func(Task) Result, workers int, logger *Logger) []Result {

	taskChan := make(chan Task, len(tasks))
	resultChan := make(chan Result, len(tasks))

	bar := progressbar.NewOptions(len(tasks),
		progressbar.OptionSetDescription("处理进度"),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskChan {
				result := workerFunc(task)
				resultChan <- result
				bar.Add(1)
			}
		}()
	}

	for _, task := range tasks {
		taskChan <- task
	}
	close(taskChan)

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var results []Result
	for result := range resultChan {
		results = append(results, result)
	}

	success := 0
	failed := 0
	var totalTime time.Duration
	for _, result := range results {
		if result.Success {
			success++
		} else {
			failed++
		}
		totalTime += result.End.Sub(result.Start)
	}

	logger.Info("\n处理完成: 成功 %d, 失败 %d, 总耗时: %v", success, failed, totalTime)

	return results
}

func GetHostFromURL(url string) string {
	url = strings.TrimPrefix(strings.TrimPrefix(url, "http://"), "https://")
	host := strings.Split(url, "/")[0]
	return host
}

func CreateOutputDir(outdir string) error {
	if err := os.MkdirAll(outdir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %v", err)
	}
	return nil
}

func ValidateURL(rawURL string) error {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return fmt.Errorf("URL 必须以 http:// 或 https:// 开头，实际得到: %q", rawURL)
	}
	return nil
}

func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func GetHostname(targetURL string) string {
	u, err := url.Parse(targetURL)
	if err != nil {
		return "unknown"
	}
	hostname := u.Hostname()
	if hostname == "" {
		return "unknown"
	}
	return hostname
}

// CreateHTTPClient 创建带有代理支持的 HTTP 客户端。
//
// 支持的代理协议（Go 1.7+ net/http.Transport 原生支持）：
//
//	http://host:port                     HTTP 代理
//	http://user:pass@host:port           HTTP 代理（带认证）
//	https://host:port                    HTTPS 代理
//	socks5://host:port                   SOCKS5 代理（本地 DNS 解析）
//	socks5://user:pass@host:port         SOCKS5 代理（带认证）
//	socks5h://host:port                  SOCKS5H 代理（由代理服务器解析 DNS）
//
// 所有协议均通过 http.Transport.Proxy = http.ProxyURL() 实现，
// 完整保留 request context 的取消/超时语义，无需额外依赖。
func CreateHTTPClient(proxyAddr string) (*http.Client, error) {
	if proxyAddr == "" {
		color.Cyan("[代理] 未设置代理，使用直连模式")
		return &http.Client{}, nil
	}

	proxyURL, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("[代理] 解析代理地址失败 %q: %v", proxyAddr, err)
	}

	scheme := strings.ToLower(proxyURL.Scheme)
	switch scheme {
	case "http", "https", "socks5", "socks5h":
		// net/http.Transport 从 Go 1.7 起原生支持 socks5/socks5h scheme，
		// 直接使用 http.ProxyURL 即可，context 取消/超时语义完整保留。
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
		client := &http.Client{Transport: transport}

		host := proxyURL.Hostname()
		port := proxyURL.Port()
		user := ""
		if proxyURL.User != nil {
			user = proxyURL.User.Username()
		}
		dnsNote := ""
		if scheme == "socks5" {
			dnsNote = " (DNS: 本地解析)"
		} else if scheme == "socks5h" {
			dnsNote = " (DNS: 远端解析)"
		}
		if user != "" {
			color.Cyan("[代理] 使用 %s 代理: %s:%s (用户: %s)%s", strings.ToUpper(scheme), host, port, user, dnsNote)
		} else {
			color.Cyan("[代理] 使用 %s 代理: %s:%s%s", strings.ToUpper(scheme), host, port, dnsNote)
		}
		return client, nil

	default:
		return nil, fmt.Errorf("[代理] 不支持的代理协议 %q，支持: http, https, socks5, socks5h", scheme)
	}
}
