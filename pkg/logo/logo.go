package logo

import (
	"fmt"

	"github.com/fatih/color"
)

func ShowLogo() {
	// 使用不同的颜色来增强视觉效果
	cyan := color.New(color.FgCyan).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	blue := color.New(color.FgBlue).SprintFunc()
	magenta := color.New(color.FgMagenta).SprintFunc()
	white := color.New(color.FgWhite).SprintFunc()

	// 为 logo 的每一行添加不同的颜色
	logo := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		cyan("  ____                          _    _ _        ____       "),
		magenta(" |  _ \\ _   _ _ __ ___  _ __   / \\  | | |      / ___| ___  "),
		yellow(" | | | | | | | '_ ' _ \\| '_ \\ / _ \\ | | |_____| |  _ / _ \\ "),
		green(" | |_| | |_| | | | | | | |_) / ___ \\| | |_____| |_| | (_) |"),
		red(" |____/ \\__,_|_| |_| |_| .__/_/   \\_\\_|_|      \\____|\\___/ "),
		white("                       |_|                                 "),
	)

	fmt.Println(logo)
	fmt.Println("Version: 1.0.0")
	fmt.Println("Author: whgojp")
	fmt.Printf("Github: %s\n", blue("https://github.com/whgojp/DumpAll-Go"))
	fmt.Println("==================================================")
}
