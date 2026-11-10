//go:build windows

// Package window shows the local interface in a native window.
package window

import (
	"fmt"

	"github.com/jchv/go-webview2"
	"github.com/jchv/go-webview2/webviewloader"
)

// Available reports whether the WebView2 runtime is present on this machine.
func Available() bool {
	version, err := webviewloader.GetInstalledVersion()
	if err != nil {
		return false
	}
	return version != ""
}

// Persistent reports whether the process must keep running after Open returns.
// The native window owns the lifetime on Windows, so it does not.
func Persistent() bool { return false }

// Open shows the interface in a native window and blocks until it closes.
func Open(url string) error {
	if !Available() {
		return fmt.Errorf("не установлен WebView2 Runtime")
	}

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "Z AI VPN",
			Width:  1200,
			Height: 780,
			Center: true,
		},
	})
	w.SetTitle("Z AI VPN")
	w.SetSize(1200, 780, webview2.HintNone)
	w.Navigate(url)
	w.Run()
	return nil
}
