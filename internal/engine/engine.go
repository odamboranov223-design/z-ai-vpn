// Package engine runs the embedded Xray-core instance in-process, so the
// application ships as a single executable with no external side-car files.
package engine

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/xtls/xray-core/common/log"
	"github.com/xtls/xray-core/core"
)

const maxLogLines = 500

// logHandler collects the engine log lines for the UI journal panel.
type logHandler struct {
	mu    sync.Mutex
	lines []string
}

func (h *logHandler) Handle(msg log.Message) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lines = append(h.lines, msg.String())
	if len(h.lines) > maxLogLines {
		h.lines = h.lines[len(h.lines)-maxLogLines:]
	}
}

func (h *logHandler) snapshot() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.lines))
	copy(out, h.lines)
	return out
}

func (h *logHandler) clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lines = nil
}

// Engine owns the running Xray instance.
type Engine struct {
	mu       sync.Mutex
	instance *core.Instance
	cancel   context.CancelFunc
	handler  *logHandler
	started  time.Time
}

// New creates an engine and installs the log collector.
func New() *Engine {
	h := &logHandler{}
	log.RegisterHandler(h)
	return &Engine{handler: h}
}

// Start loads the JSON configuration and brings the tunnel up.
func (e *Engine) Start(configJSON []byte) error {
	e.Stop()

	cfg, err := core.LoadConfig("json", bytes.NewReader(configJSON))
	if err != nil {
		return fmt.Errorf("не удалось разобрать конфигурацию: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	instance, err := core.NewWithContext(ctx, cfg)
	if err != nil {
		cancel()
		return fmt.Errorf("движок не принял конфигурацию: %w", err)
	}
	if err := instance.Start(); err != nil {
		cancel()
		return fmt.Errorf("движок не запустился: %w", err)
	}

	e.mu.Lock()
	e.instance = instance
	e.cancel = cancel
	e.started = time.Now()
	e.mu.Unlock()
	return nil
}

// Stop tears the tunnel down. It is safe to call when nothing is running.
func (e *Engine) Stop() error {
	e.mu.Lock()
	instance, cancel := e.instance, e.cancel
	e.instance, e.cancel = nil, nil
	e.mu.Unlock()

	if instance == nil {
		return nil
	}
	var err error
	if cancel != nil {
		cancel()
	}
	if instance != nil {
		err = instance.Close()
	}
	return err
}

// Running reports whether the engine is currently up.
func (e *Engine) Running() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.instance != nil
}

// Uptime returns how long the engine has been running.
func (e *Engine) Uptime() time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.instance == nil {
		return 0
	}
	return time.Since(e.started)
}

// Logs returns a copy of the collected engine log lines.
func (e *Engine) Logs() []string {
	return e.handler.snapshot()
}

// ClearLogs drops the collected log lines.
func (e *Engine) ClearLogs() {
	e.handler.clear()
}
