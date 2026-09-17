//go:build windows

package data

import (
	"os/exec"
	"syscall"
)

// yahooRunPowerShell 通过隐藏窗口的 PowerShell 发起请求（仅 Windows）。
// HideWindow 是 syscall.SysProcAttr 的 Windows 专有字段，故按平台分文件实现。
func yahooRunPowerShell(urlStr string) ([]byte, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-WindowStyle", "Hidden", "-Command",
		`try { $r = Invoke-WebRequest -Uri '`+urlStr+`' -UseBasicParsing -TimeoutSec 10 -ErrorAction Stop; Write-Output $r.Content } catch { exit 1 }`)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Output()
}
