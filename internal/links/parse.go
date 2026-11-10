package links

import (
        "encoding/base64"
        "encoding/json"
        "errors"
        "fmt"
        "net/url"
        "strconv"
        "strings"
)

// ErrEmptyLink is returned when a link has no usable payload.
var ErrEmptyLink = errors.New("links: empty link")

// Parse parses a single share link into a ServerSpec.
func Parse(raw string) (*ServerSpec, error) {
        raw = strings.TrimSpace(raw)
        if raw == "" {
                return nil, ErrEmptyLink
        }

        scheme, rest, found := strings.Cut(raw, "://")
        if !found {
                return nil, fmt.Errorf("links: missing scheme in %q", truncate(raw))
        }

        switch strings.ToLower(scheme) {
        case "vless":
                return parseVLess(rest)
        case "vmess":
                return parseVMess(rest)
        case "trojan":
                return parseTrojan(rest)
        case "ss":
                return parseShadowsocks(rest)
        case "hysteria2", "hy2":
                return parseHysteria2(rest)
        case "tuic":
                return parseTUIC(rest)
        default:
                return nil, fmt.Errorf("links: unsupported scheme %q", scheme)
        }
}

// SplitLinks extracts every share link from a blob of text. Subscription
// payloads are often base64 encoded, so the blob is decoded first when it
// does not look like plain links. Lines that carry no URL scheme at all
// (comments, blank lines) are ignored; lines that look like links but are
// malformed are returned as well so the caller can report them.
func SplitLinks(blob string) []string {
        blob = strings.TrimSpace(blob)
        if blob == "" {
                return nil
        }

        // Plain newline separated links.
        if strings.Contains(blob, "://") && !strings.Contains(blob, "\n") {
                // fall through to the per-line handling below
        }

        var candidates []string
        if !strings.Contains(blob, "://") {
                if decoded, err := base64.StdEncoding.DecodeString(blob); err == nil {
                        blob = string(decoded)
                } else if decoded, err := base64.RawURLEncoding.DecodeString(blob); err == nil {
                        blob = string(decoded)
                }
        }

        for _, line := range strings.Split(blob, "\n") {
                line = strings.TrimSpace(line)
                if line == "" || !strings.Contains(line, "://") {
                        continue
                }
                candidates = append(candidates, line)
        }
        return candidates
}

// ParseAll parses every link in a subscription payload and skips the broken
// ones instead of failing the whole batch.
func ParseAll(blob string) ([]*ServerSpec, []error) {
        var specs []*ServerSpec
        var errs []error
        for _, raw := range SplitLinks(blob) {
                spec, err := Parse(raw)
                if err != nil {
                        errs = append(errs, err)
                        continue
                }
                specs = append(specs, spec)
        }
        return specs, errs
}

// --- per-protocol parsers ---

func parseVLess(rest string) (*ServerSpec, error) {
        u, err := url.Parse("vless://" + rest)
        if err != nil {
                return nil, fmt.Errorf("links: vless: %w", err)
        }
        if u.User == nil {
                return nil, errors.New("links: vless: missing user id")
        }
        spec := &ServerSpec{
                Protocol: "vless",
                UUID:     u.User.Username(),
                Address:  u.Hostname(),
                Port:     portOf(u.Port()),
                Tag:      u.Fragment,
        }
        q := newParams(u.Query())
        spec.Flow = q.get("flow")
        spec.TLS = tlsFrom(q, spec.Flow)
        spec.Transport = transportFrom(q)
        if spec.TLS.ServerName == "" {
                spec.TLS.ServerName = spec.Transport.Host
        }
        return spec, spec.validate()
}

func parseVMess(rest string) (*ServerSpec, error) {
        payload := strings.TrimSpace(rest)
        if i := strings.Index(payload, "#"); i >= 0 {
                payload = payload[:i]
        }

        // VMessAEAD share format: vmess://<base64(security:id@host:port)>?params
        if strings.Contains(payload, "@") {
                decoded, err := decodeBase64Variants(payload)
                if err != nil {
                        return nil, fmt.Errorf("links: vmess: %w", err)
                }
                raw := "vmess://" + decoded
                u, err := url.Parse(raw)
                if err == nil && u.User != nil && u.Hostname() != "" {
                        spec := &ServerSpec{
                                Protocol: "vmess",
                                UUID:     u.User.Username(),
                                Address:  u.Hostname(),
                                Port:     portOf(u.Port()),
                                Tag:      u.Fragment,
                        }
                        q := newParams(u.Query())
                        spec.TLS = tlsFrom(q, "")
                        spec.Transport = transportFrom(q)
                        if spec.TLS.ServerName == "" {
                                spec.TLS.ServerName = spec.Transport.Host
                        }
                        return spec, spec.validate()
                }
        }

        // Classic VMess JSON payload.
        decoded, err := decodeBase64Variants(payload)
        if err != nil {
                return nil, fmt.Errorf("links: vmess: %w", err)
        }
        var raw map[string]any
        if err := json.Unmarshal([]byte(decoded), &raw); err != nil {
                return nil, fmt.Errorf("links: vmess: invalid json payload: %w", err)
        }

        spec := &ServerSpec{
                Protocol: "vmess",
                UUID:     str(raw["id"]),
                Address:  str(raw["add"]),
                Port:     intOf(raw["port"]),
                Tag:      str(raw["ps"]),
        }

        net := str(raw["net"])
        if net == "" {
                net = "tcp"
        }
        spec.Transport = Transport{
                Kind:        net,
                Path:        str(raw["path"]),
                Host:        str(raw["host"]),
                ServiceName: str(raw["path"]),
                HeaderType:  str(raw["type"]),
        }
        if net == "tcp" && str(raw["type"]) != "none" {
                spec.Transport.HeaderType = str(raw["type"])
        }

        spec.TLS = TLSOptions{
                Enabled:         strings.EqualFold(str(raw["tls"]), "tls"),
                ServerName:      str(raw["sni"]),
                ALPN:            splitList(str(raw["alpn"])),
                UTLSFingerprint: str(raw["fp"]),
        }
        if spec.TLS.ServerName == "" {
                spec.TLS.ServerName = spec.Transport.Host
        }
        return spec, spec.validate()
}

func parseTrojan(rest string) (*ServerSpec, error) {
        u, err := url.Parse("trojan://" + rest)
        if err != nil {
                return nil, fmt.Errorf("links: trojan: %w", err)
        }
        if u.User == nil {
                return nil, errors.New("links: trojan: missing password")
        }
        spec := &ServerSpec{
                Protocol: "trojan",
                Password: u.User.Username(),
                Address:  u.Hostname(),
                Port:     portOf(u.Port()),
                Tag:      u.Fragment,
        }
        q := newParams(u.Query())
        spec.TLS = tlsFrom(q, "")
        spec.Transport = transportFrom(q)
        if spec.TLS.ServerName == "" {
                spec.TLS.ServerName = spec.Transport.Host
        }
        return spec, spec.validate()
}

func parseShadowsocks(rest string) (*ServerSpec, error) {
        // Legacy format: ss://base64(method:password@host:port)
        if !strings.Contains(rest, "@") {
                decoded, err := decodeBase64Variants(strings.SplitN(rest, "#", 2)[0])
                if err != nil {
                        return nil, fmt.Errorf("links: ss: %w", err)
                }
                at := strings.LastIndex(decoded, "@")
                if at < 0 {
                        return nil, errors.New("links: ss: malformed payload")
                }
                creds, hostport := decoded[:at], decoded[at+1:]
                colon := strings.Index(creds, ":")
                if colon < 0 {
                        return nil, errors.New("links: ss: malformed credentials")
                }
                method, password := creds[:colon], creds[colon+1:]
                host, port, err := splitHostPort(hostport)
                if err != nil {
                        return nil, fmt.Errorf("links: ss: %w", err)
                }
                spec := &ServerSpec{
                        Protocol: "shadowsocks",
                        Method:   method,
                        Password: password,
                        Address:  host,
                        Port:     port,
                        Tag:      fragment(rest),
                }
                return spec, spec.validate()
        }

        // SIP002 format: ss://base64(method:password)@host:port?params#tag
        raw := "ss://" + rest
        u, err := url.Parse(raw)
        if err != nil {
                return nil, fmt.Errorf("links: ss: %w", err)
        }
        if u.User == nil {
                return nil, errors.New("links: ss: missing credentials")
        }
        method, password := "", u.User.Username()
        if pw, ok := u.User.Password(); ok {
                password = pw
        } else {
                decoded, err := decodeBase64Variants(u.User.Username())
                if err == nil {
                        if colon := strings.Index(decoded, ":"); colon >= 0 {
                                method, password = decoded[:colon], decoded[colon+1:]
                        }
                }
        }
        if method == "" {
                return nil, errors.New("links: ss: missing cipher")
        }
        spec := &ServerSpec{
                Protocol: "shadowsocks",
                Method:   method,
                Password: password,
                Address:  u.Hostname(),
                Port:     portOf(u.Port()),
                Tag:      u.Fragment,
        }
        return spec, spec.validate()
}

func parseHysteria2(rest string) (*ServerSpec, error) {
        u, err := url.Parse("hysteria2://" + rest)
        if err != nil {
                return nil, fmt.Errorf("links: hysteria2: %w", err)
        }
        spec := &ServerSpec{
                Protocol:     "hysteria2",
                Password:     u.User.Username(),
                Address:      u.Hostname(),
                Port:         portOf(u.Port()),
                Tag:          u.Fragment,
                Obfs:         "",
                ObfsPassword: "",
        }
        q := newParams(u.Query())
        spec.Obfs = q.get("obfs")
        spec.ObfsPassword = q.get("obfs-password")
        spec.UpMbps = intOf(q.get("upmbps"))
        spec.DownMbps = intOf(q.get("downmbps"))
        spec.TLS = TLSOptions{
                Enabled:    true,
                ServerName: q.get("sni"),
                Insecure:   truthy(q.get("insecure")),
        }
        if spec.TLS.ServerName == "" {
                spec.TLS.ServerName = spec.Address
        }
        return spec, spec.validate()
}

func parseTUIC(rest string) (*ServerSpec, error) {
        u, err := url.Parse("tuic://" + rest)
        if err != nil {
                return nil, fmt.Errorf("links: tuic: %w", err)
        }
        if u.User == nil {
                return nil, errors.New("links: tuic: missing credentials")
        }
        spec := &ServerSpec{
                Protocol: "tuic",
                UUID:     u.User.Username(),
                Password: "",
                Address:  u.Hostname(),
                Port:     portOf(u.Port()),
                Tag:      u.Fragment,
        }
        if pw, ok := u.User.Password(); ok {
                spec.Password = pw
        }
        q := newParams(u.Query())
        spec.CongestionControl = q.get("congestion_control")
        spec.UDPRelayMode = q.get("udp_relay_mode")
        spec.TLS = TLSOptions{
                Enabled:    true,
                ServerName: q.get("sni"),
                ALPN:       splitList(q.get("alpn")),
                Insecure:   truthy(q.get("allow_insecure")) || truthy(q.get("insecure")),
        }
        if spec.TLS.ServerName == "" {
                spec.TLS.ServerName = spec.Address
        }
        return spec, spec.validate()
}

// --- helpers ---

func (s *ServerSpec) validate() error {
        if s.Address == "" {
                return fmt.Errorf("links: %s: missing server address", s.Protocol)
        }
        if s.Port <= 0 || s.Port > 65535 {
                return fmt.Errorf("links: %s: invalid port %d", s.Protocol, s.Port)
        }
        switch s.Protocol {
        case "vless", "vmess":
                if s.UUID == "" {
                        return fmt.Errorf("links: %s: missing uuid", s.Protocol)
                }
        case "trojan", "shadowsocks":
                if s.Password == "" {
                        return fmt.Errorf("links: %s: missing password", s.Protocol)
                }
        case "hysteria2":
                if s.Password == "" {
                        return errors.New("links: hysteria2: missing auth")
                }
        case "tuic":
                if s.UUID == "" || s.Password == "" {
                        return errors.New("links: tuic: missing uuid or password")
                }
        }
        if s.Transport.Kind == "" {
                s.Transport.Kind = "tcp"
        }
        return nil
}

// params wraps url.Values with case-insensitive lookup over several aliases.
type params struct {
        values url.Values
}

func newParams(v url.Values) params { return params{values: v} }

func (p params) get(keys ...string) string {
        for _, key := range keys {
                for actual, vals := range p.values {
                        if strings.EqualFold(actual, key) && len(vals) > 0 {
                                return vals[0]
                        }
                }
        }
        return ""
}

func transportFrom(q params) Transport {
        t := Transport{Kind: strings.ToLower(q.get("type", "network", "net"))}
        if t.Kind == "" {
                t.Kind = "tcp"
        }
        t.Path = q.get("path")
        t.Host = q.get("host")
        t.ServiceName = q.get("serviceName", "servicename")
        t.Mode = q.get("mode")
        t.HeaderType = q.get("headerType", "headertype")
        return t
}

func tlsFrom(q params, flow string) TLSOptions {
        security := strings.ToLower(q.get("security"))
        tls := TLSOptions{
                Enabled:         security == "tls" || security == "reality" || security == "xtls",
                ServerName:      q.get("sni", "peer"),
                ALPN:            splitList(q.get("alpn")),
                Insecure:        truthy(q.get("allowInsecure", "allow_insecure", "insecure", "skip-cert-verify")),
                UTLSFingerprint: q.get("fp", "fingerprint"),
                // pcs (pinnedPeerCertSha256) — Quattro and other providers pass the
                // SHA-256 of the server cert as a colon-separated hex string,
                // URL-encoded with %3A for the colons. url.Query unescapes it for us.
                PinnedPeerCertSha256: q.get("pcs", "pinnedPeerCertSha256", "pin"),
        }
        if flow != "" {
                tls.Enabled = true
        }
        if security == "reality" {
                tls.Reality = &RealityOptions{
                        PublicKey: q.get("pbk", "publicKey"),
                        ShortID:   q.get("sid", "shortId"),
                        SpiderX:   q.get("spx", "spiderX"),
                }
        }
        return tls
}

func decodeBase64Variants(s string) (string, error) {
        s = strings.TrimSpace(s)
        s = strings.TrimRight(s, "=")
        encodings := []*base64.Encoding{
                base64.StdEncoding,
                base64.RawStdEncoding,
                base64.URLEncoding,
                base64.RawURLEncoding,
        }
        for _, enc := range encodings {
                if decoded, err := enc.DecodeString(s + strings.Repeat("=", (4-len(s)%4)%4)); err == nil {
                        return string(decoded), nil
                }
        }
        return "", errors.New("not valid base64")
}

func splitHostPort(hostport string) (string, int, error) {
        idx := strings.LastIndex(hostport, ":")
        if idx < 0 {
                return "", 0, errors.New("missing port")
        }
        port, err := strconv.Atoi(hostport[idx+1:])
        if err != nil {
                return "", 0, fmt.Errorf("invalid port: %w", err)
        }
        return hostport[:idx], port, nil
}

func portOf(raw string) int {
        p, _ := strconv.Atoi(raw)
        return p
}

func intOf(v any) int {
        switch t := v.(type) {
        case int:
                return t
        case int64:
                return int(t)
        case float64:
                return int(t)
        case string:
                n, _ := strconv.Atoi(strings.TrimSpace(t))
                return n
        }
        return 0
}

func str(v any) string {
        switch t := v.(type) {
        case string:
                return t
        case float64:
                return strconv.Itoa(int(t))
        }
        return ""
}

func splitList(raw string) []string {
        if raw == "" {
                return nil
        }
        parts := strings.Split(raw, ",")
        out := make([]string, 0, len(parts))
        for _, p := range parts {
                if p = strings.TrimSpace(p); p != "" {
                        out = append(out, p)
                }
        }
        return out
}

func truthy(v string) bool {
        switch strings.ToLower(strings.TrimSpace(v)) {
        case "1", "true", "yes", "on":
                return true
        }
        return false
}

func fragment(raw string) string {
        if i := strings.Index(raw, "#"); i >= 0 {
                return raw[i+1:]
        }
        return ""
}

func truncate(s string) string {
        if len(s) > 40 {
                return s[:40] + "..."
        }
        return s
}
