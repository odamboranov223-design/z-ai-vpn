package engine_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/odamboranov223-design/z-ai-vpn/internal/engine"
	"github.com/odamboranov223-design/z-ai-vpn/internal/links"
	"github.com/odamboranov223-design/z-ai-vpn/internal/xraycfg"

	"github.com/xtls/xray-core/core"

	_ "github.com/xtls/xray-core/main/distro/all"
)

const testUUID = "b8313817-3b4f-4d4e-9a1b-2c3d4e5f6a7b"

// startServer runs a local Xray instance that speaks VLESS and forwards
// everything to the given target, so the client has something real to reach.
func startServer(t *testing.T, port int) {
	t.Helper()

	cfgJSON := fmt.Sprintf(`{
      "log": {"loglevel": "warning"},
      "inbounds": [{
        "tag": "in",
        "listen": "127.0.0.1",
        "port": %d,
        "protocol": "vless",
        "settings": {"clients": [{"id": %q}], "decryption": "none"}
      }],
      "outbounds": [{"tag": "direct", "protocol": "freedom"}]
    }`, port, testUUID)

	cfg, err := core.LoadConfig("json", bytes.NewReader([]byte(cfgJSON)))
	if err != nil {
		t.Fatalf("server config: %v", err)
	}
	instance, err := core.New(cfg)
	if err != nil {
		t.Fatalf("server instance: %v", err)
	}
	if err := instance.Start(); err != nil {
		t.Fatalf("server start: %v", err)
	}
	t.Cleanup(func() { _ = instance.Close() })

	// Give the listener a moment to accept connections.
	time.Sleep(200 * time.Millisecond)
}

// TestEndToEndTunnel proves the whole chain works: a share link is parsed, a
// configuration is generated, the engine starts and traffic really flows
// through the tunnel.
func TestEndToEndTunnel(t *testing.T) {
	const (
		serverPort = 34567
		socksPort  = 34568
		httpPort   = 34569
	)

	// The "internet" the tunnel leads to.
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hello-through-the-tunnel")
	}))
	defer target.Close()

	startServer(t, serverPort)

	link := fmt.Sprintf("vless://%s@127.0.0.1:%d?encryption=none&security=none&type=tcp#local", testUUID, serverPort)
	spec, err := links.Parse(link)
	if err != nil {
		t.Fatalf("parse link: %v", err)
	}

	config, err := xraycfg.Build(spec, xraycfg.Options{
		SocksPort: socksPort,
		HTTPPort:  httpPort,
	})
	if err != nil {
		t.Fatalf("build config: %v", err)
	}

	eng := engine.New()
	if err := eng.Start(config); err != nil {
		t.Fatalf("engine start: %v", err)
	}
	defer eng.Stop()

	if !eng.Running() {
		t.Fatal("engine should be running")
	}

	proxyURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", httpPort))
	if err != nil {
		t.Fatalf("proxy url: %v", err)
	}
	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
	}

	resp, err := client.Get(target.URL)
	if err != nil {
		t.Fatalf("request through the tunnel failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "hello-through-the-tunnel" {
		t.Fatalf("unexpected body %q", body)
	}

	if eng.Uptime() <= 0 {
		t.Error("uptime should be positive while running")
	}
}

// TestEngineRejectsBrokenConfig makes sure a bad configuration fails loudly
// instead of leaving a half started tunnel behind.
func TestEngineRejectsBrokenConfig(t *testing.T) {
	eng := engine.New()

	cases := []struct {
		name string
		raw  string
	}{
		{"not json", `{"inbounds":`},
		{"unknown protocol", `{"outbounds":[{"protocol":"not-a-protocol","tag":"proxy"}]}`},
		{"unknown transport", `{"outbounds":[{"protocol":"vless","tag":"proxy","settings":{"vnext":[{"address":"127.0.0.1","port":1,"users":[{"id":"x"}]}]},"streamSettings":{"network":"telepathy"}}]}`},
	}
	for _, tc := range cases {
		if err := eng.Start([]byte(tc.raw)); err == nil {
			t.Errorf("%s: expected an error, got none", tc.name)
		}
		if eng.Running() {
			t.Errorf("%s: engine should not be running after a failed start", tc.name)
		}
	}
}

// TestEngineRestart checks that connecting twice in a row works.
func TestEngineRestart(t *testing.T) {
	const (
		serverPort = 34570
		socksPort  = 34571
		httpPort   = 34572
	)

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	}))
	defer target.Close()

	startServer(t, serverPort)

	link := fmt.Sprintf("vless://%s@127.0.0.1:%d?encryption=none&security=none&type=tcp#local", testUUID, serverPort)
	spec, err := links.Parse(link)
	if err != nil {
		t.Fatalf("parse link: %v", err)
	}
	config, err := xraycfg.Build(spec, xraycfg.Options{SocksPort: socksPort, HTTPPort: httpPort})
	if err != nil {
		t.Fatalf("build config: %v", err)
	}

	eng := engine.New()
	for i := 0; i < 2; i++ {
		if err := eng.Start(config); err != nil {
			t.Fatalf("engine start %d: %v", i, err)
		}
		proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", httpPort))
		client := &http.Client{
			Timeout:   15 * time.Second,
			Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		}
		resp, err := client.Get(target.URL)
		if err != nil {
			t.Fatalf("request %d through the tunnel failed: %v", i, err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if err := eng.Stop(); err != nil {
			t.Fatalf("engine stop %d: %v", i, err)
		}
		if eng.Running() {
			t.Fatalf("engine should be stopped after iteration %d", i)
		}
	}
}
