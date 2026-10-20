package freestockdb

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type Config struct {
	Enabled   bool
	ExePath   string
	Addr      string
	AutoStart bool
}

type Manager struct {
	cfg    Config
	client *Client

	cmd *exec.Cmd

	mu           sync.Mutex
	checkedAt    time.Time
	ok           bool
	availableTTL time.Duration
}

func NewManager(cfg Config) *Manager {
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:7899"
	}
	return &Manager{cfg: cfg, client: NewClient(cfg.Addr), availableTTL: 30 * time.Second}
}

func (m *Manager) Client() *Client { return m.client }

func (m *Manager) Start(ctx context.Context) error {
	if !m.cfg.Enabled {
		return nil
	}
	if m.client.Ping(ctx) {
		m.setOK(true)
		return nil
	}
	if !m.cfg.AutoStart || m.cfg.ExePath == "" {
		return fmt.Errorf("freestockdb: %s 未响应且未配置自动拉起", m.cfg.Addr)
	}
	cmd := exec.CommandContext(ctx, m.cfg.ExePath)
	cmd.Dir = filepath.Dir(m.cfg.ExePath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("freestockdb: 拉起 %s 失败: %w", m.cfg.ExePath, err)
	}
	m.cmd = cmd
	for i := 0; i < 10; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
		if m.client.Ping(ctx) {
			m.setOK(true)
			return nil
		}
	}
	return fmt.Errorf("freestockdb: 健康检查超时（%s）", m.cfg.Addr)
}

func (m *Manager) Stop() {
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
		m.cmd = nil
	}
}

func (m *Manager) Available(ctx context.Context) bool {
	if !m.cfg.Enabled {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if time.Since(m.checkedAt) < m.availableTTL {
		return m.ok
	}
	m.ok = m.client.Ping(ctx)
	m.checkedAt = time.Now()
	return m.ok
}

func (m *Manager) setOK(ok bool) {
	m.mu.Lock()
	m.ok, m.checkedAt = ok, time.Now()
	m.mu.Unlock()
}
