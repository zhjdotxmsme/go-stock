//go:build windows

package main

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"

	"go-stock/backend/logger"
)

// init 把进程级 stderr/stdout 重定向到 logs/panic.log。
// windowsgui 子系统没有控制台，Go 未捕获 panic（exit code 2）写 fd 2 的
// 堆栈会随空句柄丢失，表现为「闪退且无任何日志」。重定向后 panic 栈落盘可查。
func init() {
	if err := os.MkdirAll("logs", 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join("logs", "panic.log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	h := windows.Handle(f.Fd())
	if err := windows.SetStdHandle(windows.STD_ERROR_HANDLE, h); err != nil {
		logger.SugaredLogger.Warnf("重定向 stderr 到 panic.log 失败: %v", err)
		return
	}
	_ = windows.SetStdHandle(windows.STD_OUTPUT_HANDLE, h)
}
