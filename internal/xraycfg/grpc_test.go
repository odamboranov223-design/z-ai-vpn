package xraycfg

import (
	"encoding/json"
	"testing"

	"github.com/odamboranov223-design/z-ai-vpn/internal/links"
)

func TestGrpcConfigHasModeGun(t *testing.T) {
	raw := "vless://344b2810-d5ad-0007-819c-72b777e5feaa@2.27.143.66:443?encryption=none&type=grpc&serviceName=v1.segment.TestSvc&security=tls&fp=edge&sni=river-4-411.rtbcdn.ru#Latvia"
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
				Network      string                 `json:"network"`
				Security     string                 `json:"security"`
				GrpcSettings map[string]interface{} `json:"grpcSettings"`
				TlsSettings  map[string]interface{} `json:"tlsSettings"`
			} `json:"streamSettings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(cfg, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Outbounds) == 0 {
		t.Fatal("no outbounds")
	}
	stream := doc.Outbounds[0].Stream

	if stream.Network != "grpc" {
		t.Errorf("network = %q, want grpc", stream.Network)
	}
	if stream.Security != "tls" {
		t.Errorf("security = %q, want tls", stream.Security)
	}
	mode, ok := stream.GrpcSettings["mode"]
	if !ok {
		t.Errorf("grpcSettings has no 'mode' field — Xray will reject: %v", stream.GrpcSettings)
	} else if mode != "gun" {
		t.Errorf("grpcSettings.mode = %v, want gun", mode)
	}
	if sni, ok := stream.TlsSettings["serverName"]; !ok || sni != "river-4-411.rtbcdn.ru" {
		t.Errorf("tlsSettings.serverName = %v, want river-4-411.rtbcdn.ru", sni)
	}
	if fp, ok := stream.TlsSettings["fingerprint"]; !ok || fp != "edge" {
		t.Errorf("tlsSettings.fingerprint = %v, want edge", fp)
	}
	t.Logf("grpcSettings = %#v", stream.GrpcSettings)
	t.Logf("tlsSettings  = %#v", stream.TlsSettings)
}
