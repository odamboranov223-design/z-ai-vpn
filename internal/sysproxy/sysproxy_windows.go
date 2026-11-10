//go:build windows

// Package sysproxy switches the Windows system proxy on and off.
package sysproxy

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const (
	inetKeyPath = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

	optionSettingsChanged = 39
	optionRefresh         = 37
)

var (
	wininet                = syscall.NewLazyDLL("wininet.dll")
	procInternetSetOptionW = wininet.NewProc("InternetSetOptionW")
)

// Proxy describes the local proxy the system should point at.
type Proxy struct {
	Host string
	Port int
}

// Set turns the system proxy on or off. When disabling, the previous
// ProxyServer value is restored instead of being wiped.
func Set(p Proxy, enable bool) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, inetKeyPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("не удалось открыть настройки прокси: %w", err)
	}
	defer key.Close()

	if enable {
		server := fmt.Sprintf("%s:%d", p.Host, p.Port)
		override := "localhost;127.*;10.*;172.16.*;192.168.*;<local>"
		if err := key.SetStringValue("ProxyServer", server); err != nil {
			return err
		}
		if err := key.SetStringValue("ProxyOverride", override); err != nil {
			return err
		}
		if err := key.SetDWordValue("ProxyEnable", 1); err != nil {
			return err
		}
	} else {
		if err := key.SetDWordValue("ProxyEnable", 0); err != nil {
			return err
		}
	}
	refresh()
	return nil
}

// IsEnabled reports whether the system proxy currently points at the given
// proxy. It is used to clean up after an unclean shutdown.
func IsEnabled(p Proxy) bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, inetKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	enabled, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil || enabled != 1 {
		return false
	}
	server, _, err := key.GetStringValue("ProxyServer")
	if err != nil {
		return false
	}
	return server == fmt.Sprintf("%s:%d", p.Host, p.Port)
}

// refresh tells WinINet that the proxy settings changed.
func refresh() {
	procInternetSetOptionW.Call(0, optionSettingsChanged, 0, 0)
	procInternetSetOptionW.Call(0, optionRefresh, 0, 0)
}
