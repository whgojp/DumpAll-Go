package main

import (
	"dumpall-go/cmd"
	"dumpall-go/pkg/logo"
)

func main() {
	// 显示logo
	logo.ShowLogo()

	// 执行命令
	cmd.Execute()
}
