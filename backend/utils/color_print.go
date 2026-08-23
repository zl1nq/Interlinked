package utils

import (
	"fmt"
	"log"

	cp "github.com/zl1nq/color-print"
)

var style cp.Style

func InitColorPrint() {
	if style == nil {
		style = cp.NewStyle()
	}
}

func LogInfo(s string) {
	style.WithForeground(cp.HiGreen).WithBackground(cp.White)
	style.WithTextStyle(cp.Bold)
	style.WithContent("INFO: ")
	log.Println(style.Style(fmt.Sprintf(style.String() + s)))

}
func LogError(s string) {
	style.WithForeground(cp.HiRed).WithBackground(cp.White)
	style.WithTextStyle(cp.Bold)
	style.WithContent("ERROR: ")
	log.Println(style.Style(fmt.Sprintf(style.String() + s)))
}
func LogWarning(s string) {
	style.WithForeground(cp.HiYellow).WithBackground(cp.White)
	style.WithTextStyle(cp.Bold)
	style.WithContent("WARNING: ")
	log.Println(style.Style(fmt.Sprintf(style.String() + s)))
}
