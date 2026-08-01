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

	mu            sync.Mutex // 保护 cmd/checkedAt/ok
	cmd           *exec.Cmd  // 仅当由本进程拉起且健康检查通过时非空
	checkedAt     time.Time
	ok            bool
	availableTTL  time.Duration
	probeInterval time.Duration // 健康检查间隔（测试可注入）

	startMu sync.Mutex // 串行化 Start，防止并发双拉起
}

func NewManager(cfg Config) *Manager {
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:7899"
	}
	return &Manager{
		cfg:           cfg,
		client:        NewClient(cfg.Addr),
		availableTTL:  30 * time.Second,
		probeInterval: 5 * time.Second,
	}
}

func (m *Manager) Client() *Client { return m.client }

// takeCmd 取出并清空当前持有的子进程（锁内读写字段，进程操作留给调用方在锁外做）。
func (m *Manager) takeCmd() *exec.Cmd {
	m.mu.Lock()
	cmd := m.cmd
	m.cmd = nil
	m.mu.Unlock()
	return cmd
}

// killCmd 回收子进程：Kill 后 Wait 释放进程句柄。
func killCmd(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}

// Start：已在运行则直接采用；否则按配置拉起并做健康检查（probeInterval × 10 次）。
// 返回 error 时保证不留本进程拉起的后台进程。全程持有 startMu 串行化，防止并发双拉起。
func (m *Manager) Start(ctx context.Context) error {
	m.startMu.Lock()
	defer m.startMu.Unlock()
	if !m.cfg.Enabled {
		return nil
	}
	// 先探测：已运行（含本进程上次拉起的）则直接采用，不重启
	if m.client.Ping(ctx) {
		m.setOK(true)
		return nil
	}
	// 确认不响应后才回收上次拉起残留的旧实例，避免重复 Start 泄漏进程
	killCmd(m.takeCmd())
	if !m.cfg.AutoStart || m.cfg.ExePath == "" {
		return fmt.Errorf("freestockdb: %s 未响应且未配置自动拉起", m.cfg.Addr)
	}
	cmd := exec.Command(m.cfg.ExePath)
	cmd.Dir = filepath.Dir(m.cfg.ExePath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("freestockdb: 拉起 %s 失败: %w", m.cfg.ExePath, err)
	}
	for i := 0; i < 10; i++ {
		select {
		case <-ctx.Done():
			killCmd(cmd)
			return ctx.Err()
		case <-time.After(m.probeInterval):
		}
		if m.client.Ping(ctx) {
			m.mu.Lock()
			m.cmd = cmd
			m.mu.Unlock()
			m.setOK(true)
			return nil
		}
	}
	killCmd(cmd)
	return fmt.Errorf("freestockdb: 健康检查超时（%s）", m.cfg.Addr)
}

// Stop 回收由本进程拉起的 stockdb；用户自行启动的实例不动。
func (m *Manager) Stop() {
	killCmd(m.takeCmd())
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
