// Package xraycfg builds Xray-core JSON configurations from a parsed server
// specification. The generated JSON is validated against the exact Xray-core
// version the application embeds (see config_test.go).
package xraycfg

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/odamboranov223-design/z-ai-vpn/internal/links"
)

// Default local proxy ports. They match the ports used by the system proxy
// integration on Windows.
const (
	DefaultSocksPort = 10808
	DefaultHTTPPort  = 10809
)

// Options tunes the generated configuration.
type Options struct {
	// SocksPort and HTTPPort are the local inbound ports.
	SocksPort int
	HTTPPort  int
	// BypassRU routes Russian domains and IPs directly instead of through the
	// tunnel.
	BypassRU bool
	// GeoAssets reports whether geoip.dat / geosite.dat are available. When
	// they are, the bypass rules use the full geosite/geoip data set.
	GeoAssets bool
}

func (o Options) withDefaults() Options {
	if o.SocksPort == 0 {
		o.SocksPort = DefaultSocksPort
	}
	if o.HTTPPort == 0 {
		o.HTTPPort = DefaultHTTPPort
	}
	return o
}

// Build renders the full Xray configuration for the given server.
func Build(spec *links.ServerSpec, opts Options) ([]byte, error) {
	opts = opts.withDefaults()

	proxy, err := outbound(spec)
	if err != nil {
		return nil, err
	}

	sniff := map[string]any{
		"enabled":      true,
		"destOverride": []string{"http", "tls", "quic"},
		"routeOnly":    true,
	}

	cfg := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"dns": map[string]any{
			"servers": []string{"1.1.1.1", "8.8.8.8"},
		},
		"inbounds": []any{
			map[string]any{
				"tag":      "socks",
				"listen":   "127.0.0.1",
				"port":     opts.SocksPort,
				"protocol": "socks",
				"settings": map[string]any{"auth": "noauth", "udp": true},
				"sniffing": sniff,
			},
			map[string]any{
				"tag":      "http",
				"listen":   "127.0.0.1",
				"port":     opts.HTTPPort,
				"protocol": "http",
				"settings": map[string]any{},
				"sniffing": sniff,
			},
		},
		"outbounds": []any{
			proxy,
			map[string]any{"tag": "direct", "protocol": "freedom"},
			map[string]any{"tag": "block", "protocol": "blackhole"},
		},
		"routing": map[string]any{
			"domainStrategy": "AsIs",
			"rules":          routingRules(opts),
		},
	}

	return json.MarshalIndent(cfg, "", "  ")
}

// BuildFromOutbound renders a configuration around a ready made Xray outbound,
// which is how JSON configurations imported from a provider are used.
func BuildFromOutbound(raw []byte, opts Options) ([]byte, error) {
	opts = opts.withDefaults()

	var ob map[string]any
	if err := json.Unmarshal(raw, &ob); err != nil {
		return nil, fmt.Errorf("xraycfg: не удалось разобрать outbound: %w", err)
	}
	ob["tag"] = "proxy"

	sniff := map[string]any{
		"enabled":      true,
		"destOverride": []string{"http", "tls", "quic"},
		"routeOnly":    true,
	}

	cfg := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"dns": map[string]any{
			"servers": []string{"1.1.1.1", "8.8.8.8"},
		},
		"inbounds": []any{
			map[string]any{
				"tag":      "socks",
				"listen":   "127.0.0.1",
				"port":     opts.SocksPort,
				"protocol": "socks",
				"settings": map[string]any{"auth": "noauth", "udp": true},
				"sniffing": sniff,
			},
			map[string]any{
				"tag":      "http",
				"listen":   "127.0.0.1",
				"port":     opts.HTTPPort,
				"protocol": "http",
				"settings": map[string]any{},
				"sniffing": sniff,
			},
		},
		"outbounds": []any{
			ob,
			map[string]any{"tag": "direct", "protocol": "freedom"},
			map[string]any{"tag": "block", "protocol": "blackhole"},
		},
		"routing": map[string]any{
			"domainStrategy": "AsIs",
			"rules":          routingRules(opts),
		},
	}

	return json.MarshalIndent(cfg, "", "  ")
}

func routingRules(opts Options) []any {
	rules := []any{
		// Never tunnel BitTorrent traffic.
		map[string]any{"type": "field", "protocol": []string{"bittorrent"}, "outboundTag": "direct"},
	}
	if opts.GeoAssets {
		rules = append(rules, map[string]any{
			"type": "field", "ip": []string{"geoip:private"}, "outboundTag": "direct",
		})
	}
	if opts.BypassRU {
		domains := []string{"domain:ru", "domain:su", "domain:xn--p1ai"}
		var ips []string
		if opts.GeoAssets {
			domains = append([]string{"geosite:category-ru"}, domains...)
			ips = []string{"geoip:ru"}
		}
		domains = append(domains, ruDomains...)
		rules = append(rules, map[string]any{
			"type": "field", "domain": domains, "outboundTag": "direct",
		})
		if len(ips) > 0 {
			rules = append(rules, map[string]any{
				"type": "field", "ip": ips, "outboundTag": "direct",
			})
		}
	}
	return rules
}

// outbound renders the proxy outbound for the given server.
func outbound(spec *links.ServerSpec) (map[string]any, error) {
	ob := map[string]any{"tag": "proxy", "protocol": spec.Protocol}

	switch spec.Protocol {
	case "vless":
		user := map[string]any{
			"id":         spec.UUID,
			"encryption": "none",
		}
		if spec.Flow != "" {
			user["flow"] = spec.Flow
		}
		ob["settings"] = map[string]any{
			"vnext": []any{map[string]any{
				"address": spec.Address,
				"port":    spec.Port,
				"users":   []any{user},
			}},
		}
	case "vmess":
		ob["settings"] = map[string]any{
			"vnext": []any{map[string]any{
				"address": spec.Address,
				"port":    spec.Port,
				"users": []any{map[string]any{
					"id":       spec.UUID,
					"alterId":  0,
					"security": "auto",
				}},
			}},
		}
	case "trojan":
		ob["settings"] = map[string]any{
			"servers": []any{map[string]any{
				"address":  spec.Address,
				"port":     spec.Port,
				"password": spec.Password,
			}},
		}
	case "shadowsocks":
		ob["settings"] = map[string]any{
			"servers": []any{map[string]any{
				"address":  spec.Address,
				"port":     spec.Port,
				"method":   spec.Method,
				"password": spec.Password,
			}},
		}
	default:
		return nil, fmt.Errorf("xraycfg: protocol %q is not supported by the Xray engine", spec.Protocol)
	}

	stream, err := streamSettings(spec)
	if err != nil {
		return nil, err
	}
	if len(stream) > 0 {
		ob["streamSettings"] = stream
	}
	return ob, nil
}

// streamSettings renders the transport and TLS layer of the outbound.
func streamSettings(spec *links.ServerSpec) (map[string]any, error) {
	network := normalizeNetwork(spec.Transport.Kind)
	stream := map[string]any{"network": network}

	switch network {
	case "ws":
		ws := map[string]any{"path": defaultPath(spec.Transport.Path)}
		if spec.Transport.Host != "" {
			ws["host"] = spec.Transport.Host
		}
		stream["wsSettings"] = ws
	case "grpc":
		grpc := map[string]any{"serviceName": spec.Transport.ServiceName}
		if spec.Transport.Mode == "multi" {
			grpc["multiMode"] = true
		}
		stream["grpcSettings"] = grpc
	case "xhttp":
		xhttp := map[string]any{"path": defaultPath(spec.Transport.Path)}
		if spec.Transport.Host != "" {
			xhttp["host"] = spec.Transport.Host
		}
		xhttp["mode"] = spec.Transport.Mode
		if xhttp["mode"] == "" {
			xhttp["mode"] = "auto"
		}
		stream["xhttpSettings"] = xhttp
	case "httpupgrade":
		hu := map[string]any{"path": defaultPath(spec.Transport.Path)}
		if spec.Transport.Host != "" {
			hu["host"] = spec.Transport.Host
		}
		stream["httpupgradeSettings"] = hu
	case "kcp":
		stream["kcpSettings"] = map[string]any{"header": map[string]any{"type": "none"}}
	case "tcp":
		if spec.Transport.HeaderType == "http" {
			hosts := splitHosts(spec.Transport.Host)
			stream["tcpSettings"] = map[string]any{
				"header": map[string]any{
					"type": "http",
					"request": map[string]any{
						"version": "1.1",
						"method":  "GET",
						"uri":     []string{defaultPath(spec.Transport.Path)},
						"header":  headerList(hosts),
					},
				},
			}
		}
	}

	switch {
	case spec.TLS.Reality != nil:
		stream["security"] = "reality"
		reality := map[string]any{
			"serverName":  spec.TLS.ServerName,
			"fingerprint": firstNonEmpty(spec.TLS.UTLSFingerprint, "chrome"),
			"publicKey":   spec.TLS.Reality.PublicKey,
			"shortId":     spec.TLS.Reality.ShortID,
			"spiderX":     spec.TLS.Reality.SpiderX,
		}
		stream["realitySettings"] = reality
	case spec.TLS.Enabled:
		stream["security"] = "tls"
		tls := map[string]any{
			"serverName":  spec.TLS.ServerName,
			"fingerprint": spec.TLS.UTLSFingerprint,
		}
		if spec.TLS.Insecure {
			tls["allowInsecure"] = true
		}
		if len(spec.TLS.ALPN) > 0 {
			tls["alpn"] = spec.TLS.ALPN
		}
		stream["tlsSettings"] = tls
	}

	return stream, nil
}

func normalizeNetwork(kind string) string {
	switch strings.ToLower(kind) {
	case "", "tcp", "raw", "none":
		return "tcp"
	case "ws", "websocket":
		return "ws"
	case "grpc":
		return "grpc"
	case "xhttp", "splithttp", "http", "h2", "h3":
		return "xhttp"
	case "httpupgrade":
		return "httpupgrade"
	case "kcp", "mkcp":
		return "kcp"
	default:
		return "tcp"
	}
}

func defaultPath(path string) string {
	if path == "" {
		return "/"
	}
	return path
}

func splitHosts(host string) []string {
	var out []string
	for _, h := range strings.Split(host, ",") {
		if h = strings.TrimSpace(h); h != "" {
			out = append(out, h)
		}
	}
	if len(out) == 0 {
		out = []string{""}
	}
	return out
}

func headerList(hosts []string) []any {
	out := make([]any, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, map[string]any{"name": "Host", "value": []string{h}})
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// ruDomains is the fallback bypass list used when the full geosite database
// is not available. It only needs to cover the sites people actually visit.
var ruDomains = []string{
	"domain:ya.ru", "domain:yandex.ru", "domain:yandex.com",
	"domain:vk.com", "domain:vk.ru", "domain:userapi.com",
	"domain:mail.ru", "domain:list.ru", "domain:rambler.ru",
	"domain:wildberries.ru", "domain:ozon.ru", "domain:ozon.com",
	"domain:aliexpress.ru", "domain:avito.ru", "domain:avito.com",
	"domain:market.yandex.ru", "domain:gosuslugi.ru", "domain:sberbank.ru",
	"domain:sber.ru", "domain:tinkoff.ru", "domain:vtb.ru", "domain:alfabank.ru",
	"domain:megafon.ru", "domain:beeline.ru", "domain:mts.ru", "domain:telephon.ru",
	"domain:2gis.ru", "domain:hh.ru", "domain:superjob.ru", "domain:zarplata.ru",
	"domain:flamp.ru", "domain:drive2.ru", "domain:rutracker.org",
	"domain:kinopoisk.ru", "domain:ivi.ru", "domain:ok.ru", "domain:livejournal.com",
	"domain:zen.yandex.ru", "domain:dzen.ru", "domain:rambler.ru", "domain:gismedia.io",
	"domain:rg.ru", "domain:ria.ru", "domain:tass.ru", "domain:kommersant.ru",
	"domain:vc.ru", "domain:habr.com", "domain:vc.ru", "domain:stepik.org",
	"domain:skillbox.ru", "domain:netology.ru", "domain:geekbrains.ru",
	"domain:pikabu.ru", "domain:fishki.net", "domain:anekdot.ru",
	"domain:youtube.com", "domain:rutube.ru", "domain:kinopoisk.ru",
	"domain:gosuslugi.ru", "domain:nalog.ru", "domain:gosuslugi.ru",
	"domain:pochta.ru", "domain:rambler.ru", "domain:bk.ru",
	"domain:cloud.mail.ru", "domain:disk.yandex.ru", "domain:cloud.yandex.ru",
	"domain:translate.google.com", "domain:google.com", "domain:youtube.com",
}
