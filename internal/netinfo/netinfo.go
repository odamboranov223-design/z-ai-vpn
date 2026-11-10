// Package netinfo provides latency measurement and the public IP lookup used
// by the UI.
package netinfo

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Ping measures the TCP round trip time to a server in milliseconds.
func Ping(host string, port int, timeout time.Duration) (int, error) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprint(port)), timeout)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	return int(time.Since(start).Milliseconds()), nil
}

// PublicIP is the result of an IP lookup performed through the tunnel.
type PublicIP struct {
	IP      string `json:"ip"`
	Country string `json:"country"`
}

// CheckIP asks an IP echo service through the local HTTP proxy.
func CheckIP(httpPort int) (*PublicIP, error) {
	proxyURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", httpPort))
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}
	resp, err := client.Get("http://ip-api.com/json/?fields=status,countryCode,query")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body struct {
		Status      string `json:"status"`
		CountryCode string `json:"countryCode"`
		Query       string `json:"query"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	if body.Status != "success" {
		return nil, fmt.Errorf("сервис определения IP вернул %q", body.Status)
	}
	return &PublicIP{IP: body.Query, Country: body.CountryCode}, nil
}
