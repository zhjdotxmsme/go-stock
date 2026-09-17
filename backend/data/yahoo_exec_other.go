//go:build !windows

package data

import "errors"

// yahooRunPowerShell 非 Windows 平台无 PowerShell 降级通道，直接报错
//（调用方会回退到普通 HTTP 客户端路径）。
func yahooRunPowerShell(urlStr string) ([]byte, error) {
	return nil, errors.New("powershell fallback is only available on windows")
}
