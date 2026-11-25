package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func clearConsole() {
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.Command("cmd", "/c", "cls")
	} else {
		c = exec.Command("clear")
	}
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	_ = c.Run()
}

func setConsoleTitle(title string) {
	if runtime.GOOS != "windows" {
		return
	}
	cmd := exec.Command("cmd", "/c", fmt.Sprintf("title %s", title))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}
