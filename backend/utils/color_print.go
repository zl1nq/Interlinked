package utils

import (
	"fmt"
	"log"

	cp "github.com/zl1nq/color-print"
)

func LogInfo(s string) {
	style := cp.NewStyle().WithForeground(cp.HiGreen).WithBackground(cp.White)
	style.WithTextStyle(cp.Bold)
	style.WithContent("INFO: ")
	log.Println(style.Style(fmt.Sprintf(style.String() + s)))
}
func LogError(s string) {
	style := cp.NewStyle().WithForeground(cp.HiRed).WithBackground(cp.White)
	style.WithTextStyle(cp.Bold)
	style.WithContent("ERROR: ")
	log.Println(style.Style(fmt.Sprintf(style.String() + s)))
}
