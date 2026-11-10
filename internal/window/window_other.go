//go:build !windows

// Package window shows the local interface. Outside Windows the interface is
// opened in the default browser instead of a native window.
package window

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Available always reports true outside Windows.
func Available() bool { return true }

// Persistent reports whether the process must keep running after Open returns.
// The browser owns the interface outside Windows, so the server has to stay up.
func Persistent() bool { return true }

// Open opens the interface in the default browser.
func Open(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}
	if err := exec.Command(cmd, args...).Start(); err != nil {
		return fmt.Errorf("не удалось открыть браузер: %w", err)
	}
	return nil
}
