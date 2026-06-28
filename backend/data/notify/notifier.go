package notify

import "context"

// ChannelType identifies a notification channel.
type ChannelType string

const (
	// ChannelDingTalk is the DingTalk robot webhook channel.
	ChannelDingTalk ChannelType = "dingtalk"
	// ChannelWechat is the WeChat Work (企业微信) robot webhook channel.
	ChannelWechat ChannelType = "wechat"
	// ChannelFeishu is the Feishu (飞书) robot webhook channel.
	ChannelFeishu ChannelType = "feishu"
	// ChannelTelegram is the Telegram Bot API channel.
	ChannelTelegram ChannelType = "telegram"
	// ChannelEmail is the SMTP email channel.
	ChannelEmail ChannelType = "email"
)

// Message represents a notification to be sent.
type Message struct {
	Title   string // 消息标题
	Content string // 消息内容（markdown格式）
	Stock   string // 关联股票代码（可选）
}

// Notifier defines the interface for push notification channels.
type Notifier interface {
	// ChannelType returns the channel identifier.
	ChannelType() ChannelType
	// IsEnabled reports whether the channel is enabled in settings.
	IsEnabled() bool
	// Send delivers the message through the channel.
	Send(ctx context.Context, msg Message) error
}
