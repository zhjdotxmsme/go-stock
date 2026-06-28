package notify

import (
	"context"
	"errors"
	"fmt"

	"go-stock/backend/data"
)

// wxworkPayload is the request body for WeChat Work robot webhooks.
type wxworkPayload struct {
	MsgType  string         `json:"msgtype"`
	Markdown wxworkMarkdown `json:"markdown"`
}

// wxworkMarkdown is the markdown section of a WeChat Work payload.
type wxworkMarkdown struct {
	Content string `json:"content"`
}

// wxworkNotifier sends notifications through WeChat Work robot webhooks.
type wxworkNotifier struct{}

// newWxworkNotifier creates a new WeChat Work notifier.
func newWxworkNotifier() *wxworkNotifier {
	return &wxworkNotifier{}
}

// ChannelType returns the WeChat Work channel identifier.
func (w *wxworkNotifier) ChannelType() ChannelType {
	return ChannelWechat
}

// IsEnabled reports whether WeChat Work push is enabled in settings.
func (w *wxworkNotifier) IsEnabled() bool {
	return data.GetSettingConfig().WechatPushEnable
}

// Send delivers the message through WeChat Work.
func (w *wxworkNotifier) Send(ctx context.Context, msg Message) error {
	webhook := data.GetSettingConfig().WechatRobot
	if webhook == "" {
		return errors.New("wechat robot webhook is not configured")
	}

	payload := wxworkPayload{
		MsgType: "markdown",
		Markdown: wxworkMarkdown{
			Content: msg.Content,
		},
	}

	resp, err := data.SharedHTTPClient.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		Post(webhook)
	if err != nil {
		return fmt.Errorf("wechat send failed: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("wechat send failed: %s", resp.String())
	}
	return nil
}
