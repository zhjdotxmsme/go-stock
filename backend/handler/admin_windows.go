//go:build windows
// +build windows

package handler

import (
	"go-stock/backend/logger"

	"golang.org/x/sys/windows"
)

// isRunningAsAdmin 与 main 包 update_helper_windows.go 的 IsRunningAsAdmin 一致。
func isRunningAsAdmin() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		logger.SugaredLogger.Errorf("AllocateAndInitializeSid error: %s", err.Error())
		return false
	}
	defer windows.FreeSid(sid)

	token := windows.Token(0)
	h, err := windows.GetCurrentProcess()
	if err != nil {
		return false
	}
	err = windows.OpenProcessToken(h, windows.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()

	member, err := token.IsMember(sid)
	if err != nil {
		return false
	}
	return member
}
