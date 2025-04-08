package utils

import (
	"bufio"
	"fmt"
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

func ValidateURL(url string) error {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("URL必须以http://或https://开头")
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
