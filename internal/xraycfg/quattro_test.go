package xraycfg

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/odamboranov223-design/z-ai-vpn/internal/links"
)

// TestQuattroRealLink uses a real link from auth.quattro-cloud.ru subscription
// to verify the parser and config builder produce valid Xray JSON.
func TestQuattroRealLink(t *testing.T) {
	// Truncated serviceName for readability, but contains the key params
	raw := "vless://344b2810-d5ad-4eea-819c-72b777e5feaa@87.242.127.123:443?encryption=none&type=grpc&serviceName=hls-vod.iJxGWWENB3HP4nTjN8IcrA.1790085537.2512.0x5000039ce83a2b8d.2ac8ad72028049619c2fb844a825a.ca.mp4.segment-20-v1-a1.ts&mode=gun&security=tls&fp=edge&sni=river-4-411.rtbcdn.ru&pcs=28%3A6A%3A02%3A8E%3ADD%3A66%3AD9%3AE4%3AEE%3A17%3ADD%3AB0%3A32%3AC9%3A10%3A10%3AC3%3A97%3AB1%3A1D%3AE1%3A0A%3ABD%3AAB%3ABB%3A38%3ABF%3ADF%3AA8%3AB5%3AE7%3A69#%F0%9F%87%B3%F0%9F%87%B1%20Netherlands"

	spec, err := links.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	t.Logf("parsed spec:")
	t.Logf("  protocol=%s addr=%s:%d", spec.Protocol, spec.Address, spec.Port)
	t.Logf("  uuid=%s", spec.UUID)
	t.Logf("  transport.kind=%s mode=%q", spec.Transport.Kind, spec.Transport.Mode)
	t.Logf("  serviceName=%q (len=%d)", spec.Transport.ServiceName, len(spec.Transport.ServiceName))
	t.Logf("  tls.enabled=%v sni=%q fp=%q", spec.TLS.Enabled, spec.TLS.ServerName, spec.TLS.UTLSFingerprint)

	cfg, err := Build(spec, Options{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// Pretty print
	var pretty map[string]any
	if err := json.Unmarshal(cfg, &pretty); err != nil {
		t.Fatalf("generated invalid JSON: %v", err)
	}

	obs := pretty["outbounds"].([]any)
	ob := obs[0].(map[string]any)
	stream := ob["streamSettings"].(map[string]any)

	grpcBytes, _ := json.MarshalIndent(stream["grpcSettings"], "", "  ")
	tlsBytes, _ := json.MarshalIndent(stream["tlsSettings"], "", "  ")
	t.Logf("grpcSettings: %s", grpcBytes)
	t.Logf("tlsSettings:  %s", tlsBytes)

	// Check for issues
	grpc := stream["grpcSettings"].(map[string]any)
	if grpc["mode"] != "gun" {
		t.Errorf("grpcSettings.mode = %v, want gun", grpc["mode"])
	}
	if _, ok := grpc["serviceName"]; !ok {
		t.Errorf("missing serviceName")
	}

	tls := stream["tlsSettings"].(map[string]any)
	if tls["serverName"] != "river-4-411.rtbcdn.ru" {
		t.Errorf("tlsSettings.serverName = %v", tls["serverName"])
	}
	if tls["fingerprint"] != "edge" {
		t.Errorf("tlsSettings.fingerprint = %v", tls["fingerprint"])
	}
	// pcs (pinnedPeerCertSha256) is NOT currently extracted — flag it
	if _, ok := tls["pinnedPeerCertSha256"]; !ok {
		t.Logf("⚠️  pinnedPeerCertSha256 is MISSING — Quattro links have pcs= param, we should extract it")
	}

	// Verify the JSON is valid by re-parsing
	cfgStr := string(cfg)
	if !strings.Contains(cfgStr, "\"mode\": \"gun\"") {
		t.Errorf("mode:gun not in output")
	}
}
