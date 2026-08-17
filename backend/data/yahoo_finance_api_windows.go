//go:build windows

package data

import (
	"fmt"
	"os/exec"
	"syscall"
)

// yahooFetchViaPowerShell 通过 PowerShell 的 Invoke-WebRequest（WinHTTP）发起请求，
// 绕过 Go TLS 指纹被 Yahoo 限流的问题。仅 Windows 平台可用。
func (y *YahooFinanceApi) yahooFetchViaPowerShell(url string) ([]byte, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-WindowStyle", "Hidden", "-Command",
		`try { $r = Invoke-WebRequest -Uri '`+url+`' -UseBasicParsing -TimeoutSec 10 -ErrorAction Stop; Write-Output $r.Content } catch { exit 1 }`)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yahoo powershell fallback: %w", err)
	}
	return out, nil
}
