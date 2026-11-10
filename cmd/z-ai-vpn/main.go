// Command z-ai-vpn is a Windows VPN client that connects to servers given
// as share links.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/odamboranov223-design/z-ai-vpn/internal/app"
	"github.com/odamboranov223-design/z-ai-vpn/internal/ui"
	"github.com/odamboranov223-design/z-ai-vpn/internal/window"
)

func main() {
	var (
		showVersion = flag.Bool("version", false, "показать версию и выйти")
		addLink     = flag.String("add", "", "добавить сервер по ссылке и выйти")
		addSub      = flag.String("sub", "", "добавить подписку по ссылке и выйти")
		noWindow    = flag.Bool("no-window", false, "не открывать окно, только локальный адрес")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("Z AI VPN", ui.Version, "|", ui.EngineName)
		return
	}

	application, err := app.New()
	if err != nil {
		fatal(err)
	}

	if *addLink != "" {
		added, err := application.AddServers(*addLink)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("Добавлено серверов: %d\n", added)
		return
	}

	if *addSub != "" {
		if err := application.AddSubscription(*addSub, ""); err != nil {
			fatal(err)
		}
		fmt.Println("Подписка добавлена")
		return
	}

	token := randomToken()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fatal(err)
	}

	url := fmt.Sprintf("http://%s/?token=%s", listener.Addr().String(), token)
	server := &http.Server{
		Handler:           ui.Handler(application, token),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Println("Сервер интерфейса остановился:", err)
		}
	}()

	fmt.Println("Z AI VPN " + ui.Version)
	fmt.Println("Интерфейс: " + url)

	stopped := make(chan struct{})
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		<-signals
		_ = application.Disconnect()
		close(stopped)
	}()

	if *noWindow {
		fmt.Println("Работаю в фоне. Закрой окно терминала или нажми Ctrl+C для выхода.")
		<-stopped
		return
	}

	if err := window.Open(url); err != nil {
		fmt.Println("Не удалось открыть окно:", err)
		fmt.Println("Открой ссылку в браузере:", url)
		<-stopped
		return
	}

	if window.Persistent() {
		<-stopped
		return
	}

	// The native window was closed: shut the tunnel down and exit.
	_ = application.Disconnect()
	_ = server.Close()
}

func randomToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "Ошибка:", err)
	os.Exit(1)
}
