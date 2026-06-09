package main

import (
	"github.com/XIU2/CloudflareSpeedTest/gui"
	"github.com/XIU2/CloudflareSpeedTest/task"
	"github.com/XIU2/CloudflareSpeedTest/utils"
)

var (
	version = "2.2.5" // GUI version
)

func init() {
	// Initialize default values for CLI variables that task package might expect
	task.HttpingCFColomap = task.MapColoMap()
	utils.PrintNum = 10
	utils.Output = "result.csv"
}

func main() {
	gui.Start()
}
