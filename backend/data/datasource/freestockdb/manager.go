package freestockdb

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// Config free-stockdb 引擎配置（来自 Settings 表）。
type Config struct {
	Enabled   bool   // 总开关
	ExePath   string // stockdb.exe 路径（自动拉起用）
	Addr      string // 默认 127.0.0.1:7899
	AutoStart bool   // 未运行时是否自动拉起
}

// Manager 管理 stockdb 进程生命周期与可用性探测。
type Manager struct {
	cfg    Config
	client *Client

	cmd *exec.Cmd // 仅当由本进程拉起时非空

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

// Start：已在运行则直接采用；否则按配置拉起并做健康检查（5s × 10 次）。
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
	cmd := exec.Command(m.cfg.ExePath)
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

// Stop 回收由本进程拉起的 stockdb；用户自行启动的实例不动。
func (m *Manager) Stop() {
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
		m.cmd = nil
	}
}

// Available 带 30s 缓存的可用性探测（Router 每次调用都会走这里）。
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
