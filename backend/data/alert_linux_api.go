//go:build linux
// +build linux

package data

import (
	"go-stock/backend/logger"

	"github.com/gen2brain/beeep"
)

// AlertWindowsApi @Author go-stock
// @Desc Linux 平台本地通知（与 windows/darwin 同名类型保持 API 兼容）
// -----------------------------------------------------------------------------------
type AlertWindowsApi struct {
	AppID string
	// 窗口标题
	Title string
	// 窗口内容
	Content string
	// 窗口图标
	Icon string
}

func NewAlertWindowsApi(AppID string, Title string, Content string, Icon string) *AlertWindowsApi {
	return &AlertWindowsApi{
		AppID:   AppID,
		Title:   Title,
		Content: Content,
		Icon:    Icon,
	}
}

func (a AlertWindowsApi) SendNotification() bool {
	if GetSettingConfig().LocalPushEnable == false {
		return false
	}

	err := beeep.Notify(a.Title, a.Content, a.Icon)
	if err != nil {
		logger.SugaredLogger.Error(err)
		return false
	}
	return true
}
