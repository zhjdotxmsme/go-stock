//go:build !windows

package data

import "fmt"

// yahooFetchViaPowerShell 非 Windows 平台不提供 PowerShell fallback。
func (y *YahooFinanceApi) yahooFetchViaPowerShell(url string) ([]byte, error) {
	return nil, fmt.Errorf("yahoo powershell fallback: not supported on this platform")
}
