package xraycfg

import (
	"encoding/json"
	"testing"

	"github.com/odamboranov223-design/z-ai-vpn/internal/links"
)

func TestTunModeInbounds(t *testing.T) {
	spec, err := links.Parse("vless://344b2810-d5ad-0007-819c-72b777e5feaa@2.27.143.66:443?encryption=none&type=grpc&serviceName=svc&security=tls&fp=edge&sni=srv.test#Latvia")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg, err := Build(spec, Options{ConnectionMode: "tun", LocalDNS: true})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	var doc struct {
		DNS      map[string]interface{}   `json:"dns"`
		Inbounds []map[string]interface{} `json:"inbounds"`
	}
	if err := json.Unmarshal(cfg, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(doc.Inbounds) != 1 {
		t.Fatalf("want 1 inbound (tun), got %d", len(doc.Inbounds))
	}
	if doc.Inbounds[0]["protocol"] != "tun" {
		t.Errorf("inbound protocol = %v, want tun", doc.Inbounds[0]["protocol"])
	}
	settings := doc.Inbounds[0]["settings"].(map[string]interface{})
	if settings["interfaceName"] != "zaivpn0" {
		t.Errorf("interfaceName = %v, want zaivpn0", settings["interfaceName"])
	}
	if _, ok := settings["ip"]; !ok {
		t.Errorf("tun settings missing 'ip' field")
	}
	if _, ok := settings["mtu"]; !ok {
		t.Errorf("tun settings missing 'mtu' field")
	}
	// LocalDNS = true → DNS should be localhost
	servers := doc.DNS["servers"].([]interface{})
	if len(servers) != 1 || servers[0] != "localhost" {
		t.Errorf("DNS servers = %v, want [localhost]", servers)
	}
	t.Logf("tun inbound = %#v", doc.Inbounds[0])
	t.Logf("dns = %#v", doc.DNS)
}

func TestProxyModeInbounds(t *testing.T) {
	spec, err := links.Parse("vless://344b2810-d5ad-0007-819c-72b777e5feaa@2.27.143.66:443?encryption=none&type=tcp&security=tls#Latvia")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg, err := Build(spec, Options{ConnectionMode: "proxy"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	var doc struct {
		Inbounds []map[string]interface{} `json:"inbounds"`
	}
	if err := json.Unmarshal(cfg, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(doc.Inbounds) != 2 {
		t.Fatalf("want 2 inbounds (socks+http), got %d", len(doc.Inbounds))
	}
	if doc.Inbounds[0]["protocol"] != "socks" {
		t.Errorf("first inbound = %v, want socks", doc.Inbounds[0]["protocol"])
	}
	if doc.Inbounds[1]["protocol"] != "http" {
		t.Errorf("second inbound = %v, want http", doc.Inbounds[1]["protocol"])
	}
}
