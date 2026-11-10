package xraycfg

import (
	"encoding/json"
	"testing"

	"github.com/odamboranov223-design/z-ai-vpn/internal/links"
)

// TestQuattroRealityNoEmptySpiderX — REALITY config must NOT contain
// spiderX when the link doesn't provide it. An empty string "spiderX": ""
// causes Xray-core to reject the config with "Unable to load config injson".
func TestQuattroRealityNoEmptySpiderX(t *testing.T) {
	raw := "vless://344b2810-d5ad-0009-819c-72b777e5feaa@46.243.232.98:443?encryption=none&flow=xtls-rprx-vision&type=tcp&headerType=none&security=reality&fp=edge&pbk=10rVZPoOUP1TlQviIAsQ_jAROX0fRQxH0C92nq_zGQc&sid=43dcff53849b81e6&sni=river-4-402.rtbcdn.ru#Sweden"
	spec, err := links.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg, err := Build(spec, Options{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var doc struct {
		Outbounds []struct {
			Stream struct {
				RealitySettings map[string]any `json:"realitySettings"`
			} `json:"streamSettings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(cfg, &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	r := doc.Outbounds[0].Stream.RealitySettings
	if v, ok := r["spiderX"]; ok {
		t.Errorf("spiderX must be omitted when empty, got %v", v)
	}
	if r["show"] != false {
		t.Errorf("show field = %v, want false", r["show"])
	}
	if r["publicKey"] != "10rVZPoOUP1TlQviIAsQ_jAROX0fRQxH0C92nq_zGQc" {
		t.Errorf("publicKey wrong: %v", r["publicKey"])
	}
}

// TestQuattroGrpcPinnedCert — gRPC+TLS config must include
// pinnedPeerCertSha256 extracted from the pcs= URL parameter.
// Quattro and similar providers require this for TLS cert validation.
func TestQuattroGrpcPinnedCert(t *testing.T) {
	raw := "vless://344b2810-d5ad-4eea-819c-72b777e5feaa@87.242.127.123:443?encryption=none&type=grpc&serviceName=svc.test&mode=gun&security=tls&fp=edge&sni=river-4-411.rtbcdn.ru&pcs=28%3A6A%3A02%3A8E%3ADD%3A66%3AD9%3AE4%3AEE%3A17%3ADD%3AB0%3A32%3AC9%3A10%3A10%3AC3%3A97%3AB1%3A1D%3AE1%3A0A%3ABD%3AAB%3ABB%3A38%3ABF%3ADF%3AA8%3AB5%3AE7%3A69#NL"
	spec, err := links.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.TLS.PinnedPeerCertSha256 != "28:6A:02:8E:DD:66:D9:E4:EE:17:DD:B0:32:C9:10:10:C3:97:B1:1D:E1:0A:BD:AB:BB:38:BF:DF:A8:B5:E7:69" {
		t.Errorf("PinnedPeerCertSha256 not extracted: %q", spec.TLS.PinnedPeerCertSha256)
	}
	cfg, err := Build(spec, Options{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var doc struct {
		Outbounds []struct {
			Stream struct {
				GrpcSettings map[string]any `json:"grpcSettings"`
				TlsSettings  map[string]any `json:"tlsSettings"`
			} `json:"streamSettings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(cfg, &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	s := doc.Outbounds[0].Stream
	if s.GrpcSettings["mode"] != "gun" {
		t.Errorf("grpc mode = %v, want gun", s.GrpcSettings["mode"])
	}
	if _, ok := s.TlsSettings["pinnedPeerCertSha256"]; !ok {
		t.Errorf("tlsSettings missing pinnedPeerCertSha256 — TLS handshake will fail")
	}
}
