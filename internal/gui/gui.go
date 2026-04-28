package gui

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Config holds the window settings (simplified for browser-app mode)
type Config struct {
	Title  string
	URL    string
	Width  int
	Height int
}

// StartWindow opens the dashboard in a dedicated "App Mode" window.
func StartWindow(cfg Config) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "msedge", 
			fmt.Sprintf("--app=%s", cfg.URL),
			fmt.Sprintf("--window-size=%d,%d", cfg.Width, cfg.Height))
	case "darwin":
		cmd = exec.Command("open", "-a", "Google Chrome", "--args", 
			fmt.Sprintf("--app=%s", cfg.URL))
	default:
		cmd = exec.Command("google-chrome", fmt.Sprintf("--app=%s", cfg.URL))
	}

	return cmd.Start()
}

