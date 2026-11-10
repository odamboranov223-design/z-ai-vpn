package links

import "testing"

func TestParseVLessReality(t *testing.T) {
	link := "vless://b8313817-3b4f-4d4e-9a1b-2c3d4e5f6a7b@example.com:443" +
		"?encryption=none&security=reality&sni=www.microsoft.com&fp=chrome&pbk=PUBLICKEY" +
		"&sid=1a2b3c&spx=%2F&type=tcp&flow=xtls-rprx-vision#My%20Server"

	spec, err := Parse(link)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.Protocol != "vless" {
		t.Errorf("protocol = %q, want vless", spec.Protocol)
	}
	if spec.UUID != "b8313817-3b4f-4d4e-9a1b-2c3d4e5f6a7b" {
		t.Errorf("uuid = %q", spec.UUID)
	}
	if spec.Address != "example.com" || spec.Port != 443 {
		t.Errorf("address/port = %s:%d", spec.Address, spec.Port)
	}
	if spec.Tag != "My Server" {
		t.Errorf("tag = %q, want %q", spec.Tag, "My Server")
	}
	if !spec.TLS.Enabled {
		t.Error("tls should be enabled")
	}
	if spec.TLS.ServerName != "www.microsoft.com" {
		t.Errorf("sni = %q", spec.TLS.ServerName)
	}
	if spec.TLS.UTLSFingerprint != "chrome" {
		t.Errorf("fp = %q", spec.TLS.UTLSFingerprint)
	}
	if spec.TLS.Reality == nil {
		t.Fatal("reality options missing")
	}
	if spec.TLS.Reality.PublicKey != "PUBLICKEY" {
		t.Errorf("pbk = %q", spec.TLS.Reality.PublicKey)
	}
	if spec.TLS.Reality.ShortID != "1a2b3c" {
		t.Errorf("sid = %q", spec.TLS.Reality.ShortID)
	}
	if spec.Flow != "xtls-rprx-vision" {
		t.Errorf("flow = %q", spec.Flow)
	}
	if spec.Transport.Kind != "tcp" {
		t.Errorf("transport = %q", spec.Transport.Kind)
	}
}

func TestParseVLessWebsocket(t *testing.T) {
	link := "vless://uuid-1234@1.2.3.4:8443?security=tls&type=ws&host=cdn.example.org" +
		"&path=%2Fws%3Fed%3D2048&sni=cdn.example.org&alpn=h2,http%2F1.1&allowInsecure=1#WS"

	spec, err := Parse(link)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.Transport.Kind != "ws" {
		t.Errorf("transport = %q, want ws", spec.Transport.Kind)
	}
	if spec.Transport.Host != "cdn.example.org" {
		t.Errorf("host = %q", spec.Transport.Host)
	}
	if spec.Transport.Path != "/ws?ed=2048" {
		t.Errorf("path = %q", spec.Transport.Path)
	}
	if len(spec.TLS.ALPN) != 2 || spec.TLS.ALPN[0] != "h2" {
		t.Errorf("alpn = %v", spec.TLS.ALPN)
	}
	if !spec.TLS.Insecure {
		t.Error("insecure should be true")
	}
}

func TestParseVMessJSON(t *testing.T) {
	// Classic vmess:// payload: base64 of the JSON config object.
	link := "vmess://eyJ2IjoiMiIsInBzIjoiVm1Fc3MiLCJhZGQiOiJ2bWVzcy5leGFtcGxlIiwicG9ydCI6IjQ0MyIsImlkIjoiYjgzMTM4MTctM2I0Zi00ZDRlLTlhMWItMmMzZDRlNWY2YTdiIiwiYWlkIjoiMCIsInNjeSI6ImF1dG8iLCJuZXQiOiJ3cyIsInR5cGUiOiJub25lIiwiaG9zdCI6Imhvc3QuZXhhbXBsZS5jb20iLCJwYXRoIjoiL3BhdGgiLCJ0bHMiOiJ0bHMiLCJzbmkiOiJzbmkuZXhhbXBsZS5jb20ifQ=="

	spec, err := Parse(link)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.Protocol != "vmess" {
		t.Errorf("protocol = %q", spec.Protocol)
	}
	if spec.Address != "vmess.example" || spec.Port != 443 {
		t.Errorf("address/port = %s:%d", spec.Address, spec.Port)
	}
	if spec.UUID != "b8313817-3b4f-4d4e-9a1b-2c3d4e5f6a7b" {
		t.Errorf("uuid = %q", spec.UUID)
	}
	if spec.Tag != "VmEss" {
		t.Errorf("tag = %q", spec.Tag)
	}
	if spec.Transport.Kind != "ws" || spec.Transport.Path != "/path" {
		t.Errorf("transport = %+v", spec.Transport)
	}
	if !spec.TLS.Enabled || spec.TLS.ServerName != "sni.example.com" {
		t.Errorf("tls = %+v", spec.TLS)
	}
}

func TestParseTrojan(t *testing.T) {
	link := "trojan://mypassword@tr.example.net:443?security=tls&sni=tr.example.net&type=grpc&serviceName=grpcsvc#Trojan"

	spec, err := Parse(link)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.Protocol != "trojan" || spec.Password != "mypassword" {
		t.Errorf("spec = %+v", spec)
	}
	if spec.Transport.Kind != "grpc" || spec.Transport.ServiceName != "grpcsvc" {
		t.Errorf("transport = %+v", spec.Transport)
	}
}

func TestParseShadowsocksSIP002(t *testing.T) {
	// base64("aes-256-gcm:secretpass")
	link := "ss://YWVzLTI1Ni1nY206c2VjcmV0cGFzcw@1.1.1.1:8388#SS%20Node"

	spec, err := Parse(link)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.Protocol != "shadowsocks" {
		t.Errorf("protocol = %q", spec.Protocol)
	}
	if spec.Method != "aes-256-gcm" {
		t.Errorf("method = %q", spec.Method)
	}
	if spec.Password != "secretpass" {
		t.Errorf("password = %q", spec.Password)
	}
	if spec.Address != "1.1.1.1" || spec.Port != 8388 {
		t.Errorf("address/port = %s:%d", spec.Address, spec.Port)
	}
}

func TestParseShadowsocksLegacy(t *testing.T) {
	// base64("chacha20-ietf-poly1305:pw@2.2.2.2:9000")
	link := "ss://Y2hhY2hhMjAtaWV0Zi1wb2x5MTMwNTpwd0AyLjIuMi4yOjkwMDA=#Legacy"

	spec, err := Parse(link)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.Method != "chacha20-ietf-poly1305" || spec.Password != "pw" {
		t.Errorf("credentials = %q / %q", spec.Method, spec.Password)
	}
	if spec.Address != "2.2.2.2" || spec.Port != 9000 {
		t.Errorf("address/port = %s:%d", spec.Address, spec.Port)
	}
}

func TestParseHysteria2(t *testing.T) {
	link := "hysteria2://myauth@hy2.example.com:443?sni=hy2.example.com&insecure=1" +
		"&obfs=salamander&obfs-password=obfspass#HY2"

	spec, err := Parse(link)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.Protocol != "hysteria2" || spec.Password != "myauth" {
		t.Errorf("spec = %+v", spec)
	}
	if spec.Obfs != "salamander" || spec.ObfsPassword != "obfspass" {
		t.Errorf("obfs = %q / %q", spec.Obfs, spec.ObfsPassword)
	}
	if !spec.TLS.Insecure {
		t.Error("insecure should be true")
	}
}

func TestParseTUIC(t *testing.T) {
	link := "tuic://uuid-9876:pass-word@tuic.example.com:443?sni=tuic.example.com" +
		"&alpn=h3&congestion_control=bbr&udp_relay_mode=native#TUIC"

	spec, err := Parse(link)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.Protocol != "tuic" || spec.UUID != "uuid-9876" || spec.Password != "pass-word" {
		t.Errorf("spec = %+v", spec)
	}
	if spec.CongestionControl != "bbr" || spec.UDPRelayMode != "native" {
		t.Errorf("cc/udp = %q / %q", spec.CongestionControl, spec.UDPRelayMode)
	}
}

func TestSplitLinksFromBase64Subscription(t *testing.T) {
	// base64("vless://uuid-1234@example.com:443\ntrojan://pass@example.com:443")
	raw := "dmxlc3M6Ly91dWlkLTEyMzRAZXhhbXBsZS5jb206NDQzCnRyb2phbjovL3Bhc3NAZXhhbXBsZS5jb206NDQz"

	specs, errs := ParseAll(raw)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(specs) != 2 {
		t.Fatalf("parsed %d links, want 2", len(specs))
	}
	if specs[0].Protocol != "vless" || specs[1].Protocol != "trojan" {
		t.Errorf("protocols = %q, %q", specs[0].Protocol, specs[1].Protocol)
	}
}

func TestParseAllKeepsGoingOnBrokenLink(t *testing.T) {
	// Lines without a scheme are ignored outright; a line that looks like a
	// link but is malformed must be reported as an error without stopping the
	// rest of the batch.
	blob := "vless://uuid@example.com:443\n# a comment\nvless://broken\ntrojan://pass@example.com:443"
	specs, errs := ParseAll(blob)
	if len(specs) != 2 {
		t.Fatalf("parsed %d links, want 2", len(specs))
	}
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1", len(errs))
	}
}

func TestParseRejectsBadInput(t *testing.T) {
	cases := []string{
		"",
		"notalink",
		"http://example.com",
		"vless://",
		"vless://uuid@host:99999",
		"trojan://",
	}
	for _, c := range cases {
		if _, err := Parse(c); err == nil {
			t.Errorf("Parse(%q) = nil error, want error", c)
		}
	}
}
