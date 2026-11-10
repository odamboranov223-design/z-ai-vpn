//go:build windows

// Package window shows the local interface in a native window.
//
// Z AI VPN: frameless window with rounded corners (Windows 11),
// custom HTML title bar, and JS bindings for window controls.
package window

import (
        "fmt"
        "syscall"
        "unsafe"

        "github.com/jchv/go-webview2"
        "github.com/jchv/go-webview2/webviewloader"
)

// ─── Win32 lazy procs ───────────────────────────────────────────────
var (
        user32 = syscall.NewLazyDLL("user32.dll")
        dwmapi = syscall.NewLazyDLL("dwmapi.dll")

        pGetWindowLongPtrW = user32.NewProc("GetWindowLongPtrW")
        pSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
        pSetWindowPos      = user32.NewProc("SetWindowPos")
        pSendMessageW      = user32.NewProc("SendMessageW")
        pReleaseCapture    = user32.NewProc("ReleaseCapture")
        pShowWindowAsync   = user32.NewProc("ShowWindowAsync")
        pIsWindowVisible   = user32.NewProc("IsWindowVisible")
        pGetWindowRect     = user32.NewProc("GetWindowRect")
        pDwmSetWindowAttr  = dwmapi.NewProc("DwmSetWindowAttribute")
)

// Window style flags. Negative constants can't be cast directly to
// uintptr (it's unsigned), so GWL_* use the ^uintptr(N) trick:
// ^uintptr(15) == 0xFFF...F0 == -16 in two's complement.
const (
        GWL_STYLE   = ^uintptr(15) // -16
        GWL_EXSTYLE = ^uintptr(19) // -20

        WS_POPUP       = uintptr(0x80000000)
        WS_VISIBLE     = uintptr(0x10000000)
        WS_CAPTION     = uintptr(0x00C00000)
        WS_THICKFRAME  = uintptr(0x00040000)
        WS_MINIMIZEBOX = uintptr(0x00020000)
        WS_MAXIMIZEBOX = uintptr(0x00010000)
        WS_SYSMENU     = uintptr(0x00080000)

        SWP_NOSIZE     = 0x0001
        SWP_NOMOVE     = 0x0002
        SWP_NOZORDER   = 0x0004
        SWP_NOACTIVATE = 0x0010
        SWP_FRAMECHANGED = 0x0020

        WM_NCLBUTTONDOWN = 0xA1
        HTCAPTION        = 2

        SW_MINIMIZE       = 6
        SW_MAXIMIZE       = 3
        SW_RESTORE        = 9
        SW_SHOWNORMAL     = 1

        // DWM window corner preference (Windows 11)
        DWMWA_WINDOW_CORNER_PREFERENCE = 33
        DWMWCP_DEFAULT                 = 0
        DWMWCP_DONOTROUND              = 1
        DWMWCP_ROUND                   = 2
        DWMWCP_ROUNDSMALL              = 3

        // DWMWA_CAPTION_COLOR (Windows 11 22000+) — set caption to black
        DWMWA_CAPTION_COLOR = 35
        // DWMWA_TEXT_COLOR — set caption text color
        DWMWA_TEXT_COLOR = 36
        // DWMWA_BORDER_COLOR
        DWMWA_BORDER_COLOR = 34
        // DWMWA_USE_IMMERSIVE_DARK_MODE = 20 (Windows 10 1809+)
        DWMWA_USE_IMMERSIVE_DARK_MODE = 20
)

type rect struct {
        Left, Top, Right, Bottom int32
}

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

// Open shows the interface in a frameless native window and blocks until closed.
func Open(url string) error {
        if !Available() {
                return fmt.Errorf("не установлен WebView2 Runtime")
        }

        w := webview2.NewWithOptions(webview2.WebViewOptions{
                AutoFocus: true,
                WindowOptions: webview2.WindowOptions{
                        Title:  "Z AI VPN",
                        Width:  1200,
                        Height: 800,
                        Center: true,
                },
        })
        if w == nil {
                return fmt.Errorf("не удалось создать окно")
        }

        hwnd := w.Window()
        applyFramelessStyle(hwnd)
        bindWindowControls(w, hwnd)

        w.SetTitle("Z AI VPN")
        w.SetSize(1200, 800, webview2.HintNone)
        w.Navigate(url)
        w.Run()
        return nil
}

// applyFramelessStyle removes the standard caption, sets rounded corners
// (Windows 11), enables dark mode, and forces a frame redraw.
func applyFramelessStyle(hwnd unsafe.Pointer) {
        h := uintptr(hwnd)

        // 1. Strip the standard caption / title bar but keep resize + min/max.
        style, _, _ := pGetWindowLongPtrW.Call(h, uintptr(GWL_STYLE))
        style &^= WS_CAPTION | WS_SYSMENU
        style |= WS_THICKFRAME | WS_MINIMIZEBOX | WS_MAXIMIZEBOX
        pSetWindowLongPtrW.Call(h, uintptr(GWL_STYLE), style)

        // 2. Dark mode on the (now hidden) non-client area, for clean edges.
        dark := int32(1) // TRUE
        pDwmSetWindowAttr.Call(h, uintptr(DWMWA_USE_IMMERSIVE_DARK_MODE),
                uintptr(unsafe.Pointer(&dark)), unsafe.Sizeof(dark))

        // 3. Rounded corners (Windows 11).
        round := int32(DWMWCP_ROUND)
        pDwmSetWindowAttr.Call(h, uintptr(DWMWA_WINDOW_CORNER_PREFERENCE),
                uintptr(unsafe.Pointer(&round)), unsafe.Sizeof(round))

        // 4. Force the new frame to apply.
        pSetWindowPos.Call(h, 0, 0, 0, 0, 0,
                uintptr(SWP_NOMOVE|SWP_NOSIZE|SWP_NOZORDER|SWP_NOACTIVATE|SWP_FRAMECHANGED))
}

// bindWindowControls exposes minimize / maximize-toggle / close / drag to JS.
//
//      windowMinimize()      -> minimize the window
//      windowToggleMax()     -> maximize or restore depending on state
//      windowClose()         -> destroy the window and exit
//      windowDrag()          -> start a native caption drag
//
// They are called from the custom HTML title bar.
func bindWindowControls(w webview2.WebView, hwnd unsafe.Pointer) {
        h := uintptr(hwnd)

        _ = w.Bind("windowMinimize", func() error {
                pShowWindowAsync.Call(h, uintptr(SW_MINIMIZE))
                return nil
        })

        _ = w.Bind("windowToggleMax", func() error {
                // Toggle between maximized and restored.
                style, _, _ := pGetWindowLongPtrW.Call(h, uintptr(GWL_STYLE))
                if style&WS_MAXIMIZEBOX == 0 {
                        return nil
                }
                // Detect current maximized state via window rect vs work area
                var r rect
                pGetWindowRect.Call(h, uintptr(unsafe.Pointer(&r)))
                // If the window currently has WS_MAXIMIZE in its actual placement
                // (we can't read WS_MAXIMIZE directly via GetWindowLong because that
                // reports style flags, not state), toggle by sending a restore or
                // maximize command.
                // Simpler heuristic: track via SW_RESTORE first; if already restored,
                // SW_RESTORE is a no-op so follow up with SW_MAXIMIZE.
                // Use IsZoomed via user32.
                isZoomed, _, _ := user32.NewProc("IsZoomed").Call(h)
                if isZoomed != 0 {
                        pShowWindowAsync.Call(h, uintptr(SW_RESTORE))
                } else {
                        pShowWindowAsync.Call(h, uintptr(SW_MAXIMIZE))
                }
                return nil
        })

        _ = w.Bind("windowClose", func() error {
                w.Destroy()
                return nil
        })

        _ = w.Bind("windowDrag", func() error {
                pReleaseCapture.Call()
                pSendMessageW.Call(h, uintptr(WM_NCLBUTTONDOWN), uintptr(HTCAPTION), 0)
                return nil
        })
}
