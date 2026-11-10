//go:build !windows

// Package sysproxy switches the Windows system proxy on and off. On other
// platforms every operation is a no-op.
package sysproxy

// Proxy describes the local proxy the system should point at.
type Proxy struct {
	Host string
	Port int
}

// Set is a no-op outside Windows.
func Set(p Proxy, enable bool) error { return nil }

// IsEnabled always reports false outside Windows.
func IsEnabled(p Proxy) bool { return false }
