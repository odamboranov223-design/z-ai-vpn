package ui_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/odamboranov223-design/z-ai-vpn/internal/app"
	"github.com/odamboranov223-design/z-ai-vpn/internal/ui"
)

func newServer(t *testing.T, token string) *httptest.Server {
	t.Helper()
	application, err := app.New()
	if err != nil {
		t.Fatalf("app: %v", err)
	}
	srv := httptest.NewServer(ui.Handler(application, token))
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, srv *httptest.Server, path, token string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if token != "" {
		req.Header.Set("X-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestStaticAssetsAreServed(t *testing.T) {
	srv := newServer(t, "tok")

	for _, path := range []string{"/", "/app.css", "/app.js"} {
		code, body := get(t, srv, path, "")
		if code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", path, code)
		}
		if len(body) == 0 {
			t.Errorf("%s: empty body", path)
		}
	}
}

func TestAPIIsProtectedByToken(t *testing.T) {
	srv := newServer(t, "secret")

	code, _ := get(t, srv, "/api/state", "")
	if code != http.StatusForbidden {
		t.Errorf("without token: status = %d, want 403", code)
	}

	code, body := get(t, srv, "/api/state", "secret")
	if code != http.StatusOK {
		t.Errorf("with token: status = %d, want 200", code)
	}

	var state map[string]any
	if err := json.Unmarshal([]byte(body), &state); err != nil {
		t.Fatalf("state is not json: %v", err)
	}
	if _, ok := state["connected"]; !ok {
		t.Error("state should report the connection flag")
	}
}

func TestServersAndSettingsEndpoints(t *testing.T) {
	srv := newServer(t, "secret")

	code, body := get(t, srv, "/api/servers", "secret")
	if code != http.StatusOK {
		t.Fatalf("servers: status = %d", code)
	}
	var servers []map[string]any
	if err := json.Unmarshal([]byte(body), &servers); err != nil {
		t.Fatalf("servers is not a json array: %v", err)
	}

	code, body = get(t, srv, "/api/settings", "secret")
	if code != http.StatusOK {
		t.Fatalf("settings: status = %d", code)
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(body), &settings); err != nil {
		t.Fatalf("settings is not json: %v", err)
	}
	for _, key := range []string{"bypassRU", "sysProxy", "checkIP", "subscriptions"} {
		if _, ok := settings[key]; !ok {
			t.Errorf("settings should contain %q", key)
		}
	}
}

func TestAddServerEndpoint(t *testing.T) {
	srv := newServer(t, "secret")

	payload := `{"text":"vless://b8313817-3b4f-4d4e-9a1b-2c3d4e5f6a7b@example.com:443?encryption=none&security=tls&type=tcp#Test"}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/servers/add", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Token", "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d: %s", resp.StatusCode, body)
	}

	code, body := get(t, srv, "/api/servers", "secret")
	if code != http.StatusOK {
		t.Fatalf("servers: status = %d", code)
	}
	if !strings.Contains(body, "example.com") {
		t.Errorf("the added server should appear in the list, got: %s", body)
	}
}
