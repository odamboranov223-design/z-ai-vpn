package xraycfg

import (
	"bytes"
	"strings"
	"testing"

	"github.com/odamboranov223-design/z-ai-vpn/internal/links"

	"github.com/xtls/xray-core/core"

	_ "github.com/xtls/xray-core/main/distro/all"
)

// validate parses and instantiates the config with the real Xray-core, which
// is the only way to be sure the generated JSON matches the engine.
func validate(t *testing.T, raw []byte) {
	t.Helper()
	cfg, err := core.LoadConfig("json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("xray rejected the config: %v\n%s", err, raw)
	}
	instance, err := core.New(cfg)
	if err != nil {
		t.Fatalf("xray failed to instantiate: %v\n%s", err, raw)
	}
	if err := instance.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func build(t *testing.T, link string, opts Options) []byte {
	t.Helper()
	spec, err := links.Parse(link)
	if err != nil {
		t.Fatalf("parse %q: %v", link, err)
	}
	raw, err := Build(spec, opts)
	if err != nil {
		t.Fatalf("build %q: %v", link, err)
	}
	return raw
}

func TestBuildVLESSReality(t *testing.T) {
	link := "vless://b8313817-3b4f-4d4e-9a1b-2c3d4e5f6a7b@example.com:443" +
		"?encryption=none&security=reality&sni=www.microsoft.com&fp=chrome" +
		"&pbk=06-IF7LRjJHjLilvj8JyZaNsmNsJuMysuB3S7cNl3Og" +
		"&sid=1a2b3c&spx=%2F&type=tcp&flow=xtls-rprx-vision#Reality"
	raw := build(t, link, Options{})
	if !strings.Contains(string(raw), `"realitySettings"`) {
		t.Errorf("realitySettings missing:\n%s", raw)
	}
	if !strings.Contains(string(raw), `"flow": "xtls-rprx-vision"`) {
		t.Errorf("flow missing:\n%s", raw)
	}
	validate(t, raw)
}

func TestBuildVLESSWebsocket(t *testing.T) {
	link := "vless://uuid-1234@1.2.3.4:8443?security=tls&type=ws&host=cdn.example.org" +
		"&path=%2Fws&sni=cdn.example.org&alpn=h2%2Chttp%2F1.1#WS"
	raw := build(t, link, Options{})
	if !strings.Contains(string(raw), `"wsSettings"`) {
		t.Errorf("wsSettings missing:\n%s", raw)
	}
	validate(t, raw)
}

func TestBuildVLESSGRPC(t *testing.T) {
	link := "vless://uuid-1234@1.2.3.4:443?security=tls&type=grpc&serviceName=svc&mode=multi#GRPC"
	raw := build(t, link, Options{})
	if !strings.Contains(string(raw), `"multiMode": true`) {
		t.Errorf("multiMode missing:\n%s", raw)
	}
	validate(t, raw)
}

func TestBuildVLESSXHTTP(t *testing.T) {
	link := "vless://uuid-1234@1.2.3.4:443?security=tls&type=xhttp&path=%2Fxh&host=h.example&mode=stream-one#XHTTP"
	raw := build(t, link, Options{})
	if !strings.Contains(string(raw), `"xhttpSettings"`) {
		t.Errorf("xhttpSettings missing:\n%s", raw)
	}
	validate(t, raw)
}

func TestBuildVLESSHTTPUpgrade(t *testing.T) {
	link := "vless://uuid-1234@1.2.3.4:443?security=tls&type=httpupgrade&path=%2Fu&host=h.example#HU"
	raw := build(t, link, Options{})
	if !strings.Contains(string(raw), `"httpupgradeSettings"`) {
		t.Errorf("httpupgradeSettings missing:\n%s", raw)
	}
	validate(t, raw)
}

func TestBuildVMess(t *testing.T) {
	link := "vmess://eyJ2IjoiMiIsInBzIjoiVm1Fc3MiLCJhZGQiOiJ2bWVzcy5leGFtcGxlIiwicG9ydCI6IjQ0MyIsImlkIjoiYjgzMTM4MTctM2I0Zi00ZDRlLTlhMWItMmMzZDRlNWY2YTdiIiwiYWlkIjoiMCIsInNjeSI6ImF1dG8iLCJuZXQiOiJ3cyIsInR5cGUiOiJub25lIiwiaG9zdCI6Imhvc3QuZXhhbXBsZS5jb20iLCJwYXRoIjoiL3BhdGgiLCJ0bHMiOiJ0bHMiLCJzbmkiOiJzbmkuZXhhbXBsZS5jb20ifQ=="
	raw := build(t, link, Options{})
	if !strings.Contains(string(raw), `"protocol": "vmess"`) {
		t.Errorf("protocol missing:\n%s", raw)
	}
	validate(t, raw)
}

func TestBuildTrojan(t *testing.T) {
	link := "trojan://mypassword@tr.example.net:443?security=tls&sni=tr.example.net#Trojan"
	raw := build(t, link, Options{})
	if !strings.Contains(string(raw), `"protocol": "trojan"`) {
		t.Errorf("protocol missing:\n%s", raw)
	}
	validate(t, raw)
}

func TestBuildShadowsocks(t *testing.T) {
	link := "ss://YWVzLTI1Ni1nY206c2VjcmV0cGFzcw@1.1.1.1:8388#SS"
	raw := build(t, link, Options{})
	if !strings.Contains(string(raw), `"method": "aes-256-gcm"`) {
		t.Errorf("method missing:\n%s", raw)
	}
	validate(t, raw)
}

func TestBuildUnsupportedProtocol(t *testing.T) {
	spec, err := links.Parse("hysteria2://auth@hy2.example.com:443?sni=hy2.example.com#HY2")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := Build(spec, Options{}); err == nil {
		t.Fatal("expected an error for a protocol the Xray engine cannot handle")
	}
}

func TestBypassRules(t *testing.T) {
	link := "trojan://pw@example.com:443?security=tls#T"
	raw := build(t, link, Options{BypassRU: true})
	s := string(raw)
	if !strings.Contains(s, `"outboundTag": "direct"`) {
		t.Errorf("direct rule missing:\n%s", raw)
	}
	if !strings.Contains(s, "domain:ru") {
		t.Errorf("ru domain rule missing:\n%s", raw)
	}
	validate(t, raw)
}
