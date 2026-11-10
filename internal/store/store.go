// Package store persists the user's subscriptions, servers and settings.
package store

import (
        "encoding/json"
        "os"
        "path/filepath"
        "sync"
        "time"

        "github.com/odamboranov223-design/z-ai-vpn/internal/links"
)

// Server is one proxy server the user can connect to.
type Server struct {
        ID          string            `json:"id"`
        Name        string            `json:"name"`
        Link        string            `json:"link"`
        Spec        *links.ServerSpec `json:"spec"`
        RawOutbound json.RawMessage   `json:"rawOutbound,omitempty"`
        Source      string            `json:"source"`
        SubID       string            `json:"subId"`
        SubName     string            `json:"subName"`
        Country     string            `json:"country"`
        CountryCode string            `json:"countryCode"`
        Ping        *int              `json:"ping"`
}

// Subscription is a remote list of servers.
type Subscription struct {
        ID         string    `json:"id"`
        Name       string    `json:"name"`
        URL        string    `json:"url"`
        Traffic    int64     `json:"traffic"`
        Used       int64     `json:"used"`
        Expiry     time.Time `json:"expiry"`
        LastUpdate time.Time `json:"lastUpdate"`
        Servers    int       `json:"serverCount"`
}

// Settings holds the user preferences.
type Settings struct {
        BypassRU  bool   `json:"bypassRU"`
        SysProxy  bool   `json:"sysProxy"`
        CheckIP   bool   `json:"checkIP"`
        TrayMode  bool   `json:"trayMode"`
        Theme     string `json:"theme"`
        // ConnectionMode is "proxy" (system HTTP/SOCKS proxy) or "tun"
        // (Xray TUN virtual adapter, captures all system traffic).
        // TUN mode requires admin privileges on Windows.
        ConnectionMode string `json:"connectionMode"`
        // LocalDNS makes Xray resolve DNS locally instead of through the tunnel.
        LocalDNS bool `json:"localDNS"`
}

// Data is the whole persisted state.
type Data struct {
        Subscriptions []Subscription `json:"subscriptions"`
        Servers       []Server       `json:"servers"`
        Settings      Settings       `json:"settings"`
}

func defaultData() Data {
        return Data{
                Subscriptions: []Subscription{},
                Servers:       []Server{},
                Settings: Settings{
                        BypassRU:       true,
                        SysProxy:       true,
                        CheckIP:        true,
                        Theme:          "dark",
                        ConnectionMode: "proxy",
                        LocalDNS:       false,
                },
        }
}

// Store is a JSON file backed store.
type Store struct {
        mu   sync.RWMutex
        path string
        data Data
}

// Dir returns the directory the application stores its data in.
func Dir() string {
        base, err := os.UserConfigDir()
        if err != nil || base == "" {
                home, herr := os.UserHomeDir()
                if herr != nil {
                        return filepath.Join(os.TempDir(), "ZAIVPN")
                }
                base = home
        }
        return filepath.Join(base, "ZAIVPN")
}

// Open loads the store from disk, creating an empty one when missing.
func Open() (*Store, error) {
        dir := Dir()
        if err := os.MkdirAll(dir, 0o755); err != nil {
                return nil, err
        }
        s := &Store{path: filepath.Join(dir, "data.json"), data: defaultData()}

        raw, err := os.ReadFile(s.path)
        if err != nil {
                if os.IsNotExist(err) {
                        return s, nil
                }
                return nil, err
        }
        var loaded Data
        if err := json.Unmarshal(raw, &loaded); err != nil {
                // A corrupt file must not brick the app: keep the backup and start fresh.
                _ = os.Rename(s.path, s.path+".broken")
                return s, nil
        }
        if loaded.Subscriptions == nil {
                loaded.Subscriptions = []Subscription{}
        }
        if loaded.Servers == nil {
                loaded.Servers = []Server{}
        }
        if loaded.Settings.Theme == "" {
                loaded.Settings.Theme = "dark"
        }
        s.data = loaded
        return s, nil
}

// Save writes the current state back to disk atomically.
func (s *Store) Save() error {
        s.mu.RLock()
        raw, err := json.MarshalIndent(s.data, "", "  ")
        s.mu.RUnlock()
        if err != nil {
                return err
        }
        tmp := s.path + ".tmp"
        if err := os.WriteFile(tmp, raw, 0o644); err != nil {
                return err
        }
        return os.Rename(tmp, s.path)
}

// Data returns a copy of the whole state.
func (s *Store) Data() Data {
        s.mu.RLock()
        defer s.mu.RUnlock()
        return s.snapshotLocked()
}

func (s *Store) snapshotLocked() Data {
        out := Data{
                Subscriptions: append([]Subscription{}, s.data.Subscriptions...),
                Servers:       append([]Server{}, s.data.Servers...),
                Settings:      s.data.Settings,
        }
        return out
}

// Settings returns the current settings.
func (s *Store) Settings() Settings {
        s.mu.RLock()
        defer s.mu.RUnlock()
        return s.data.Settings
}

// UpdateSettings applies the given changes to the settings.
func (s *Store) UpdateSettings(fn func(*Settings)) {
        s.mu.Lock()
        fn(&s.data.Settings)
        s.mu.Unlock()
        _ = s.Save()
}

// Servers returns a copy of the server list.
func (s *Store) Servers() []Server {
        s.mu.RLock()
        defer s.mu.RUnlock()
        return append([]Server{}, s.data.Servers...)
}

// Subscriptions returns a copy of the subscription list.
func (s *Store) Subscriptions() []Subscription {
        s.mu.RLock()
        defer s.mu.RUnlock()
        return append([]Subscription{}, s.data.Subscriptions...)
}

// UpsertSubscription inserts or replaces a subscription by URL.
func (s *Store) UpsertSubscription(sub Subscription) Subscription {
        s.mu.Lock()
        for i := range s.data.Subscriptions {
                if s.data.Subscriptions[i].URL == sub.URL {
                        sub.ID = s.data.Subscriptions[i].ID
                        s.data.Subscriptions[i] = sub
                        s.mu.Unlock()
                        _ = s.Save()
                        return sub
                }
        }
        s.data.Subscriptions = append(s.data.Subscriptions, sub)
        s.mu.Unlock()
        _ = s.Save()
        return sub
}

// DeleteSubscription removes a subscription and all servers that came from it.
func (s *Store) DeleteSubscription(id string) {
        s.mu.Lock()
        kept := s.data.Subscriptions[:0]
        for _, sub := range s.data.Subscriptions {
                if sub.ID != id {
                        kept = append(kept, sub)
                }
        }
        s.data.Subscriptions = kept

        servers := s.data.Servers[:0]
        for _, srv := range s.data.Servers {
                if srv.SubID != id {
                        servers = append(servers, srv)
                }
        }
        s.data.Servers = servers
        s.mu.Unlock()
        _ = s.Save()
}

// ReplaceServersForSubscription swaps the server list that belongs to a
// subscription, keeping servers added by hand untouched.
func (s *Store) ReplaceServersForSubscription(subID string, servers []Server) {
        s.mu.Lock()
        kept := make([]Server, 0, len(s.data.Servers)+len(servers))
        for _, srv := range s.data.Servers {
                if srv.SubID != subID {
                        kept = append(kept, srv)
                }
        }
        kept = append(kept, servers...)
        s.data.Servers = kept
        s.mu.Unlock()
        _ = s.Save()
}

// AddServers appends manually imported servers, skipping duplicates.
func (s *Store) AddServers(servers []Server) int {
        s.mu.Lock()
        existing := map[string]bool{}
        for _, srv := range s.data.Servers {
                existing[srv.Link] = true
        }
        added := 0
        for _, srv := range servers {
                if srv.Link != "" && existing[srv.Link] {
                        continue
                }
                if srv.Link != "" {
                        existing[srv.Link] = true
                }
                s.data.Servers = append(s.data.Servers, srv)
                added++
        }
        s.mu.Unlock()
        _ = s.Save()
        return added
}

// DeleteServer removes a single server by ID.
func (s *Store) DeleteServer(id string) {
        s.mu.Lock()
        kept := s.data.Servers[:0]
        for _, srv := range s.data.Servers {
                if srv.ID != id {
                        kept = append(kept, srv)
                }
        }
        s.data.Servers = kept
        s.mu.Unlock()
        _ = s.Save()
}

// SetPing records a latency measurement for a server.
func (s *Store) SetPing(id string, ping int) {
        s.mu.Lock()
        for i := range s.data.Servers {
                if s.data.Servers[i].ID == id {
                        p := ping
                        s.data.Servers[i].Ping = &p
                        break
                }
        }
        s.mu.Unlock()
        _ = s.Save()
}
