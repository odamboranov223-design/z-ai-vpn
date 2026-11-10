// Package ui serves the local web interface and the JSON API the interface
// talks to. Everything is bound to the loopback interface only.
package ui

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/odamboranov223-design/z-ai-vpn/internal/app"
	"github.com/odamboranov223-design/z-ai-vpn/internal/store"
)

//go:embed assets
var assets embed.FS

// Handler builds the HTTP handler for the local UI.
func Handler(a *app.App, token string) http.Handler {
	mux := http.NewServeMux()

	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err)
	}

	mux.Handle("/", http.FileServer(http.FS(sub)))

	api := func(fn func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !authorized(r, token) {
				writeError(w, http.StatusForbidden, "не авторизован")
				return
			}
			fn(w, r)
		}
	}

	mux.HandleFunc("/api/state", api(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.State())
	}))

	mux.HandleFunc("/api/servers", api(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.Servers())
	}))

	mux.HandleFunc("/api/log", api(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.Engine().Logs())
	}))

	mux.HandleFunc("/api/settings", api(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			settings := a.Store().Settings()
			subs := a.Store().Subscriptions()
			writeJSON(w, map[string]any{
				"bypassRU":      settings.BypassRU,
				"sysProxy":      settings.SysProxy,
				"checkIP":       settings.CheckIP,
				"trayMode":      settings.TrayMode,
				"theme":         settings.Theme,
				"subscriptions": subs,
				"version":       Version,
				"engine":        EngineName,
				"dataDir":       store.Dir(),
				"ports":         a.State().Ports,
			})
			return
		}
		var payload store.Settings
		if err := readJSON(r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		a.UpdateSettings(payload)
		writeJSON(w, map[string]any{"ok": true})
	}))

	mux.HandleFunc("/api/connect", api(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			ID string `json:"id"`
		}
		if err := readJSON(r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := a.Connect(payload.ID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true, "message": "Подключено"})
	}))

	mux.HandleFunc("/api/disconnect", api(func(w http.ResponseWriter, r *http.Request) {
		if err := a.Disconnect(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true, "message": "Отключено"})
	}))

	mux.HandleFunc("/api/ping", api(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			ID string `json:"id"`
		}
		_ = readJSON(r, &payload)
		message, err := a.Ping(payload.ID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true, "message": message})
	}))

	mux.HandleFunc("/api/servers/add", api(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Text string `json:"text"`
		}
		if err := readJSON(r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		added, err := a.AddServers(payload.Text)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]any{
			"ok":      true,
			"added":   added,
			"message": "Добавлено серверов: " + strconv.Itoa(added),
		})
	}))

	mux.HandleFunc("/api/servers/delete", api(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			ID string `json:"id"`
		}
		if err := readJSON(r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		a.DeleteServer(payload.ID)
		writeJSON(w, map[string]any{"ok": true})
	}))

	mux.HandleFunc("/api/subscriptions/add", api(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			URL  string `json:"url"`
			Name string `json:"name"`
		}
		if err := readJSON(r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := a.AddSubscription(payload.URL, payload.Name); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true, "message": "Подписка добавлена"})
	}))

	mux.HandleFunc("/api/subscriptions/delete", api(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			ID string `json:"id"`
		}
		if err := readJSON(r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		a.DeleteSubscription(payload.ID)
		writeJSON(w, map[string]any{"ok": true})
	}))

	mux.HandleFunc("/api/subscriptions/refresh", api(func(w http.ResponseWriter, r *http.Request) {
		if err := a.RefreshSubscriptions(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true, "message": "Подписки обновлены"})
	}))

	return mux
}

// Version is the application version, overridden at build time.
var Version = "0.0.0-dev"

// EngineName is the name of the embedded proxy engine.
const EngineName = "Xray-core (встроен)"

// authorized checks the shared token. The token is sent as a query parameter
// for the page and as a header for API calls.
func authorized(r *http.Request, token string) bool {
	if token == "" {
		return true
	}
	if r.Header.Get("X-Token") == token {
		return true
	}
	return r.URL.Query().Get("token") == token
}

func readJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(target); err != nil && err.Error() != "EOF" {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// StaticFS exposes the embedded assets, which is handy for tests.
func StaticFS() fs.FS {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err)
	}
	return sub
}

// TrimToken removes the token parameter from a URL path.
func TrimToken(path string) string {
	if i := strings.Index(path, "?"); i >= 0 {
		return path[:i]
	}
	return path
}
