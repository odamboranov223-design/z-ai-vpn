// Package app wires the store, the proxy engine and the system proxy together
// and exposes the operations the UI needs.
package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/odamboranov223-design/z-ai-vpn/internal/country"
	"github.com/odamboranov223-design/z-ai-vpn/internal/engine"
	"github.com/odamboranov223-design/z-ai-vpn/internal/links"
	"github.com/odamboranov223-design/z-ai-vpn/internal/netinfo"
	"github.com/odamboranov223-design/z-ai-vpn/internal/store"
	"github.com/odamboranov223-design/z-ai-vpn/internal/subscription"
	"github.com/odamboranov223-design/z-ai-vpn/internal/sysproxy"
	"github.com/odamboranov223-design/z-ai-vpn/internal/xraycfg"
)

// Ports used by the local proxy.
const (
	SocksPort = xraycfg.DefaultSocksPort
	HTTPPort  = xraycfg.DefaultHTTPPort
)

// App is the application service.
type App struct {
	mu      sync.Mutex
	store   *store.Store
	engine  *engine.Engine
	busy    bool
	current *store.Server
	ip      string
	ipCC    string
}

// New creates the application service and cleans up a stale system proxy left
// behind by an unclean shutdown.
func New() (*App, error) {
	st, err := store.Open()
	if err != nil {
		return nil, err
	}
	a := &App{store: st, engine: engine.New()}
	if sysproxy.IsEnabled(sysproxy.Proxy{Host: "127.0.0.1", Port: HTTPPort}) {
		_ = sysproxy.Set(sysproxy.Proxy{Host: "127.0.0.1", Port: HTTPPort}, false)
	}
	return a, nil
}

// Store exposes the underlying store.
func (a *App) Store() *store.Store { return a.store }

// Engine exposes the proxy engine.
func (a *App) Engine() *engine.Engine { return a.engine }

// ServerView is the JSON shape of a server for the UI.
type ServerView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
	FlagEmoji   string `json:"flagEmoji"`
	FlagURL     string `json:"flagUrl"`
	Protocol    string `json:"protocol"`
	Transport   string `json:"transport"`
	Security    string `json:"security"`
	Address     string `json:"address"`
	Port        int    `json:"port"`
	Ping        *int   `json:"ping"`
	SubID       string `json:"subId"`
	SubName     string `json:"subName"`
	Traffic     string `json:"traffic"`
	Expiry      string `json:"expiry"`
}

// StateView is the JSON shape of the connection state.
type StateView struct {
	Connected bool        `json:"connected"`
	Busy      bool        `json:"busy"`
	CurrentID string      `json:"currentId"`
	Current   *ServerView `json:"current"`
	IP        string      `json:"ip"`
	Country   string      `json:"country"`
	Ports     string      `json:"ports"`
}

// Servers returns the server list rendered for the UI.
func (a *App) Servers() []ServerView {
	servers := a.store.Servers()
	subs := map[string]store.Subscription{}
	for _, sub := range a.store.Subscriptions() {
		subs[sub.ID] = sub
	}

	views := make([]ServerView, 0, len(servers))
	for _, srv := range servers {
		views = append(views, a.viewFor(srv, subs))
	}
	return views
}

func (a *App) viewFor(srv store.Server, subs map[string]store.Subscription) ServerView {
	view := ServerView{
		ID:          srv.ID,
		Name:        srv.Name,
		Country:     srv.Country,
		CountryCode: srv.CountryCode,
		FlagEmoji:   country.Emoji(srv.CountryCode),
		FlagURL:     country.FlagURL(srv.CountryCode),
		Protocol:    "",
		Address:     "",
		Ping:        srv.Ping,
		SubID:       srv.SubID,
		SubName:     srv.SubName,
	}

	if srv.Spec != nil {
		view.Protocol = srv.Spec.Protocol
		view.Address = srv.Spec.Address
		view.Port = srv.Spec.Port
		view.Transport = srv.Spec.Transport.Kind
		switch {
		case srv.Spec.TLS.Reality != nil:
			view.Security = "reality"
		case srv.Spec.TLS.Enabled:
			view.Security = "tls"
		default:
			view.Security = "none"
		}
	} else if len(srv.RawOutbound) > 0 {
		var ob map[string]any
		if err := json.Unmarshal(srv.RawOutbound, &ob); err == nil {
			view.Protocol, _ = ob["protocol"].(string)
			if addr, port := outboundAddress(ob); addr != "" {
				view.Address = addr
				view.Port = port
			}
		}
	}

	if sub, ok := subs[srv.SubID]; ok {
		view.Traffic = formatTraffic(sub.Traffic, sub.Used)
		view.Expiry = formatExpiry(sub.Expiry)
	}
	return view
}

func outboundAddress(ob map[string]any) (string, int) {
	settings, _ := ob["settings"].(map[string]any)
	if settings == nil {
		return "", 0
	}
	for _, key := range []string{"vnext", "servers"} {
		list, _ := settings[key].([]any)
		if len(list) == 0 {
			continue
		}
		first, _ := list[0].(map[string]any)
		addr, _ := first["address"].(string)
		switch port := first["port"].(type) {
		case float64:
			return addr, int(port)
		case string:
			n, _ := strconv.Atoi(port)
			return addr, n
		}
	}
	return "", 0
}

// State returns the current connection state.
func (a *App) State() StateView {
	a.mu.Lock()
	defer a.mu.Unlock()

	view := StateView{
		Busy:    a.busy,
		IP:      a.ip,
		Country: a.ipCC,
		Ports:   fmt.Sprintf("SOCKS %d · HTTP %d", SocksPort, HTTPPort),
	}
	if a.current != nil {
		view.Connected = true
		view.CurrentID = a.current.ID
		subs := map[string]store.Subscription{}
		for _, sub := range a.store.Subscriptions() {
			subs[sub.ID] = sub
		}
		current := a.viewFor(*a.current, subs)
		view.Current = &current
	}
	return view
}

// Connect brings the tunnel up for the given server.
func (a *App) Connect(id string) error {
	a.mu.Lock()
	if a.busy {
		a.mu.Unlock()
		return fmt.Errorf("приложение занято")
	}
	a.busy = true
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.busy = false
		a.mu.Unlock()
	}()

	servers := a.store.Servers()
	var target *store.Server
	for i := range servers {
		if servers[i].ID == id {
			target = &servers[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("сервер не найден")
	}

	settings := a.store.Settings()
	opts := xraycfg.Options{
		SocksPort: SocksPort,
		HTTPPort:  HTTPPort,
		BypassRU:  settings.BypassRU,
	}

	var config []byte
	var err error
	if len(target.RawOutbound) > 0 {
		config, err = xraycfg.BuildFromOutbound(target.RawOutbound, opts)
	} else if target.Spec != nil {
		config, err = xraycfg.Build(target.Spec, opts)
	} else {
		err = fmt.Errorf("у сервера нет параметров подключения")
	}
	if err != nil {
		return err
	}

	if err := a.engine.Start(config); err != nil {
		return err
	}

	if settings.SysProxy {
		_ = sysproxy.Set(sysproxy.Proxy{Host: "127.0.0.1", Port: HTTPPort}, true)
	}

	a.mu.Lock()
	a.current = target
	a.ip, a.ipCC = "", ""
	a.mu.Unlock()

	if settings.CheckIP {
		if info, err := netinfo.CheckIP(HTTPPort); err == nil {
			a.mu.Lock()
			a.ip, a.ipCC = info.IP, info.Country
			a.mu.Unlock()
		}
	}
	return nil
}

// Disconnect tears the tunnel down.
func (a *App) Disconnect() error {
	a.mu.Lock()
	a.busy = true
	a.mu.Unlock()

	err := a.engine.Stop()
	if a.store.Settings().SysProxy {
		_ = sysproxy.Set(sysproxy.Proxy{Host: "127.0.0.1", Port: HTTPPort}, false)
	}

	a.mu.Lock()
	a.current = nil
	a.ip, a.ipCC = "", ""
	a.busy = false
	a.mu.Unlock()
	return err
}

// AddServers parses the given text and stores the servers it contains. The text
// may be a list of links, a base64 subscription body or an Xray JSON config.
func (a *App) AddServers(text string) (int, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, fmt.Errorf("пусто")
	}

	added := 0
	var errs []string

	if text[0] == '{' || text[0] == '[' {
		payload := subscription.DecodeJSON(text)
		for _, ob := range payload.RawOutbounds {
			srv := a.serverFromOutbound(ob, payload.Remarks[string(ob)], "")
			a.store.AddServers([]store.Server{srv})
			added++
		}
		if added > 0 {
			return added, nil
		}
	}

	specs, parseErrs := links.ParseAll(text)
	for _, err := range parseErrs {
		errs = append(errs, err.Error())
	}
	if len(specs) == 0 {
		if len(errs) > 0 {
			return 0, fmt.Errorf("%s", strings.Join(errs, "; "))
		}
		return 0, fmt.Errorf("не удалось распознать ни одной ссылки")
	}

	servers := make([]store.Server, 0, len(specs))
	for _, spec := range specs {
		servers = append(servers, a.serverFromSpec(spec, "", ""))
	}
	added = a.store.AddServers(servers)
	return added, nil
}

func (a *App) serverFromSpec(spec *links.ServerSpec, subID, subName string) store.Server {
	name := spec.DisplayName()
	code, cname := country.Detect(name)
	if cname == "" {
		cname = spec.Address
	}
	return store.Server{
		ID:          newID(),
		Name:        name,
		Link:        "",
		Spec:        spec,
		Source:      "manual",
		SubID:       subID,
		SubName:     subName,
		Country:     cname,
		CountryCode: code,
	}
}

func (a *App) serverFromOutbound(raw json.RawMessage, remarks, subID string) store.Server {
	srv := store.Server{
		ID:          newID(),
		RawOutbound: raw,
		Source:      "manual",
		SubID:       subID,
		SubName:     remarks,
	}
	var ob map[string]any
	if err := json.Unmarshal(raw, &ob); err == nil {
		addr, port := outboundAddress(ob)
		srv.Name = remarks
		if srv.Name == "" {
			srv.Name = fmt.Sprintf("%s:%d", addr, port)
		}
		code, cname := country.Detect(srv.Name)
		srv.Country, srv.CountryCode = cname, code
	}
	return srv
}

// DeleteServer removes a server.
func (a *App) DeleteServer(id string) {
	a.mu.Lock()
	current := a.current != nil && a.current.ID == id
	a.mu.Unlock()
	if current {
		_ = a.Disconnect()
	}
	a.store.DeleteServer(id)
}

// Ping measures the latency of one server, or of every server when id is empty.
func (a *App) Ping(id string) (string, error) {
	servers := a.store.Servers()
	if id == "" {
		go a.pingAll(servers)
		return "Пинг запущен", nil
	}
	for _, srv := range servers {
		if srv.ID != id {
			continue
		}
		if srv.Spec == nil {
			return "У сервера нет параметров подключения", nil
		}
		ms, err := netinfo.Ping(srv.Spec.Address, srv.Spec.Port, 5*time.Second)
		if err != nil {
			a.store.SetPing(srv.ID, -1)
			return "Сервер недоступен", nil
		}
		a.store.SetPing(srv.ID, ms)
		return fmt.Sprintf("%d мс", ms), nil
	}
	return "Сервер не найден", nil
}

func (a *App) pingAll(servers []store.Server) {
	for _, srv := range servers {
		if srv.Spec == nil {
			continue
		}
		ms, err := netinfo.Ping(srv.Spec.Address, srv.Spec.Port, 4*time.Second)
		if err != nil {
			a.store.SetPing(srv.ID, -1)
			continue
		}
		a.store.SetPing(srv.ID, ms)
	}
}

// AddSubscription downloads a subscription and stores the servers it returns.
func (a *App) AddSubscription(url, name string) error {
	payload, info, err := subscription.Fetch(url)
	if err != nil {
		return err
	}
	if name == "" {
		name = info.Title
	}
	if name == "" {
		name = url
	}

	sub := store.Subscription{
		ID:         newID(),
		Name:       name,
		URL:        url,
		Traffic:    info.Traffic,
		Used:       info.Used,
		Expiry:     info.Expiry,
		LastUpdate: time.Now(),
	}
	sub = a.store.UpsertSubscription(sub)

	servers := a.serversFromPayload(payload, sub)
	a.store.ReplaceServersForSubscription(sub.ID, servers)
	sub.Servers = len(servers)
	a.store.UpsertSubscription(sub)
	return nil
}

// RefreshSubscriptions re-downloads every subscription.
func (a *App) RefreshSubscriptions() error {
	subs := a.store.Subscriptions()
	if len(subs) == 0 {
		return fmt.Errorf("нет подписок")
	}
	var lastErr error
	for _, sub := range subs {
		payload, info, err := subscription.Fetch(sub.URL)
		if err != nil {
			lastErr = err
			continue
		}
		sub.Traffic, sub.Used, sub.Expiry = info.Traffic, info.Used, info.Expiry
		sub.LastUpdate = time.Now()
		if info.Title != "" {
			sub.Name = info.Title
		}
		a.store.UpsertSubscription(sub)
		servers := a.serversFromPayload(payload, sub)
		a.store.ReplaceServersForSubscription(sub.ID, servers)
		sub.Servers = len(servers)
		a.store.UpsertSubscription(sub)
	}
	return lastErr
}

func (a *App) serversFromPayload(payload *subscription.Payload, sub store.Subscription) []store.Server {
	var servers []store.Server
	for _, raw := range payload.Links {
		spec, err := links.Parse(raw)
		if err != nil {
			continue
		}
		srv := a.serverFromSpec(spec, sub.ID, sub.Name)
		srv.Source = sub.URL
		srv.Link = raw
		servers = append(servers, srv)
	}
	for _, ob := range payload.RawOutbounds {
		srv := a.serverFromOutbound(ob, payload.Remarks[string(ob)], sub.ID)
		srv.Source = sub.URL
		servers = append(servers, srv)
	}
	return servers
}

// DeleteSubscription removes a subscription and its servers.
func (a *App) DeleteSubscription(id string) {
	a.mu.Lock()
	current := a.current != nil && a.current.SubID == id
	a.mu.Unlock()
	if current {
		_ = a.Disconnect()
	}
	a.store.DeleteSubscription(id)
}

// UpdateSettings applies the given settings.
func (a *App) UpdateSettings(s store.Settings) {
	a.store.UpdateSettings(func(cur *store.Settings) {
		cur.BypassRU = s.BypassRU
		cur.SysProxy = s.SysProxy
		cur.CheckIP = s.CheckIP
		cur.TrayMode = s.TrayMode
		if s.Theme != "" {
			cur.Theme = s.Theme
		}
	})
}

func newID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buf)
}

func formatTraffic(total, used int64) string {
	if total <= 0 {
		return ""
	}
	return fmt.Sprintf("%s из %s", humanBytes(used), humanBytes(total))
}

func formatExpiry(t time.Time) string {
	if t.IsZero() || t.Unix() <= 0 {
		return ""
	}
	return t.Local().Format("02.01.2006")
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d Б", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
