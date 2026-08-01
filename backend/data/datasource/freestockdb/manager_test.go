package freestockdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestManagerAvailableAdoptsRunningInstance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"0":["000001"]}`))
	}))
	defer srv.Close()
	m := NewManager(Config{Enabled: true, Addr: srv.URL[len("http://"):]})
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !m.Available(context.Background()) {
		t.Error("should adopt already-running instance")
	}
	m.Stop() // 不应 panic（非本进程拉起）
}

func TestManagerDisabled(t *testing.T) {
	m := NewManager(Config{Enabled: false})
	if m.Available(context.Background()) {
		t.Error("disabled manager must be unavailable")
	}
}

func TestManagerUnavailableNoAutoStart(t *testing.T) {
	m := NewManager(Config{Enabled: true, Addr: "127.0.0.1:1"})
	if err := m.Start(context.Background()); err == nil {
		t.Error("Start should fail when nothing listening and AutoStart off")
	}
	if m.Available(context.Background()) {
		t.Error("should be unavailable")
	}
}

func TestManagerAvailableCache(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	m := NewManager(Config{Enabled: true, Addr: srv.URL[len("http://"):]})
	m.Available(context.Background())
	m.Available(context.Background()) // 30s 内第二次应走缓存
	if hits != 1 {
		t.Errorf("hits = %d, want 1 (cached)", hits)
	}
	// 测试可注入较短缓存窗口
	m.availableTTL = time.Nanosecond
	time.Sleep(time.Millisecond)
	m.Available(context.Background())
	if hits != 2 {
		t.Errorf("hits = %d, want 2 after ttl", hits)
	}
}

func TestManagerStartFailureReapsSpawnedProcess(t *testing.T) {
	// 假 exe：cmd.Start() 能成功，但永不监听 Addr，健康检查必然超时
	m := NewManager(Config{
		Enabled:   true,
		Addr:      "127.0.0.1:1",
		AutoStart: true,
		ExePath:   `C:\Windows\System32\ping.exe`,
	})
	m.probeInterval = time.Millisecond
	if err := m.Start(context.Background()); err == nil {
		t.Fatal("Start should fail health check against dead port")
	}
	if m.cmd != nil {
		t.Error("m.cmd must be nil after failed Start (no leaked process)")
	}
	m.Stop() // 不应 panic
}

func TestManagerConcurrentStartStop(t *testing.T) {
	// 无 gcc 环境跑不了 -race，用并发压测替代：Start/Stop/Available 并发调用不应 panic/死锁
	m := NewManager(Config{Enabled: true, Addr: "127.0.0.1:1"})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = m.Start(context.Background())
				m.Stop()
				m.Available(context.Background())
			}
		}()
	}
	wg.Wait()
}
