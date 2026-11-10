// Package subscription downloads and decodes proxy subscription payloads.
package subscription

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Info carries the metadata a provider exposes in the response headers.
type Info struct {
	// Traffic is the total traffic allowance in bytes, zero when unknown.
	Traffic int64
	// Used is the consumed traffic in bytes.
	Used int64
	// Expiry is the moment the subscription runs out.
	Expiry time.Time
	// Title is the provider name advertised by the server.
	Title string
}

// Payload is a decoded subscription.
type Payload struct {
	// Links are the share links found in the payload.
	Links []string
	// RawOutbounds are ready made Xray outbounds found in a JSON config.
	RawOutbounds []json.RawMessage
	// Remarks is the provider name attached to the JSON configs.
	Remarks map[string]string
}

// Fetch downloads a subscription. It tries a couple of user agents because
// some providers serve different content depending on the client.
func Fetch(url string) (*Payload, Info, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	var lastErr error
	for _, ua := range []string{"v2rayN/7.0", "Clash.Meta/1.18", "Mozilla/5.0"} {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, Info{}, err
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept", "*/*")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("подписка вернула HTTP %d", resp.StatusCode)
			continue
		}
		if readErr != nil {
			lastErr = readErr
			continue
		}

		info := Info{
			Title: decodeTitle(resp.Header.Get("profile-title")),
		}
		if u, ok := parseUserInfo(resp.Header.Get("subscription-userinfo")); ok {
			info.Traffic, info.Used, info.Expiry = u[0], u[1], time.Unix(u[2], 0)
		}
		return decode(string(body)), info, nil
	}
	return nil, Info{}, lastErr
}

// decode turns a raw subscription body into links and outbounds.
func decode(body string) *Payload {
	body = strings.TrimSpace(strings.TrimPrefix(body, "\ufeff"))
	if body == "" {
		return &Payload{Links: []string{}, Remarks: map[string]string{}}
	}

	// A JSON document is either a single config or an array of configs.
	if body[0] == '{' || body[0] == '[' {
		if payload := decodeJSON(body); payload != nil {
			return payload
		}
	}

	// Otherwise it may be a base64 encoded list of links.
	if !strings.Contains(body, "://") {
		if decoded, err := base64.StdEncoding.DecodeString(body); err == nil {
			body = string(decoded)
		} else if decoded, err := base64.RawStdEncoding.DecodeString(body); err == nil {
			body = string(decoded)
		}
	}

	payload := &Payload{Links: []string{}, Remarks: map[string]string{}}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "://") {
			continue
		}
		payload.Links = append(payload.Links, line)
	}
	return payload
}

// DecodeJSON decodes an Xray JSON configuration (or an array of them) into
// the proxy outbounds it contains.
func DecodeJSON(body string) *Payload {
	return decodeJSON(body)
}

// decodeJSON extracts the proxy outbounds of one or more Xray configs.
func decodeJSON(body string) *Payload {
	var raw any
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return nil
	}
	payload := &Payload{Links: []string{}, Remarks: map[string]string{}}

	var configs []any
	switch v := raw.(type) {
	case []any:
		configs = v
	case map[string]any:
		configs = []any{v}
	default:
		return nil
	}

	for _, c := range configs {
		cfg, ok := c.(map[string]any)
		if !ok {
			continue
		}
		remarks, _ := cfg["remarks"].(string)
		outbounds, _ := cfg["outbounds"].([]any)
		for _, o := range outbounds {
			ob, ok := o.(map[string]any)
			if !ok {
				continue
			}
			proto, _ := ob["protocol"].(string)
			switch proto {
			case "vless", "vmess", "trojan", "shadowsocks":
			default:
				continue
			}
			encoded, err := json.Marshal(ob)
			if err != nil {
				continue
			}
			payload.RawOutbounds = append(payload.RawOutbounds, encoded)
			payload.Remarks[string(encoded)] = remarks
		}
	}
	return payload
}

// decodeTitle reads the profile title header, which is sometimes base64.
func decodeTitle(raw string) string {
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "base64:") {
		if decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(raw, "base64:")); err == nil {
			return string(decoded)
		}
		return ""
	}
	return raw
}

// parseUserInfo reads the subscription-userinfo header, which carries the
// traffic counters and the expiry timestamp.
func parseUserInfo(raw string) ([3]int64, bool) {
	var out [3]int64
	if raw == "" {
		return out, false
	}
	for _, part := range strings.Split(raw, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		value, err := strconv.ParseInt(strings.TrimSpace(kv[1]), 10, 64)
		if err != nil {
			continue
		}
		switch strings.TrimSpace(kv[0]) {
		case "total":
			out[0] = value
		case "download":
			out[1] = value
		case "expire":
			out[2] = value
		}
	}
	return out, out[0] > 0 || out[1] > 0 || out[2] > 0
}
