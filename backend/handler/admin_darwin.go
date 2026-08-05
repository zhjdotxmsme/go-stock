//go:build darwin
// +build darwin

package handler

// isRunningAsAdmin 与 main 包 update_helper_darwin.go 的 IsRunningAsAdmin 一致。
func isRunningAsAdmin() bool {
	return true
}
