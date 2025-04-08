package dumper

import (
	"net/http"
)

// ProgressCallback 定义进度回调函数类型
type ProgressCallback func(url string, statusCode int, filePath string)

// Dumper 接口定义了所有子命令需要实现的方法
type Dumper interface {
	// Execute 执行下载操作
	Execute(url string, outdir string, proxy string, force bool, debug bool, workers int, progressCb ProgressCallback) error
	// Validate 验证URL是否有效
	Validate(url string) error
	// Check 检查目标是否存在信息泄露
	Check(url string, client *http.Client) (bool, error)
	// GetName 获取命令名称
	GetName() string
	// GetDescription 获取命令描述
	GetDescription() string
}

// BaseDumper 提供基础实现
type BaseDumper struct {
	Name        string
	Description string
}

// GetName 获取命令名称
func (d *BaseDumper) GetName() string {
	return d.Name
}

// GetDescription 获取命令描述
func (d *BaseDumper) GetDescription() string {
	return d.Description
}
