//go:build linux
// +build linux

package handler

import "os"

// isRunningAsAdmin 与 main 包 update_helper_linux.go 的 IsRunningAsAdmin 一致。
func isRunningAsAdmin() bool {
	return os.Geteuid() == 0
}
