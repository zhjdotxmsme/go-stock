package notify

import (
	"context"
	"errors"
	"strings"

	"go-stock/backend/data"
)

// dingtalkNotifier sends notifications through DingTalk robot webhooks.
type dingtalkNotifier struct{}

// newDingTalkNotifier creates a new DingTalk notifier.
func newDingTalkNotifier() *dingtalkNotifier {
	return &dingtalkNotifier{}
}

// ChannelType returns the DingTalk channel identifier.
func (d *dingtalkNotifier) ChannelType() ChannelType {
	return ChannelDingTalk
}

// IsEnabled reports whether DingTalk push is enabled in settings.
func (d *dingtalkNotifier) IsEnabled() bool {
	return data.GetSettingConfig().DingPushEnable
}

// Send delivers the message through DingTalk.
func (d *dingtalkNotifier) Send(ctx context.Context, msg Message) error {
	if data.GetSettingConfig().DingRobot == "" {
		return errors.New("dingtalk robot webhook is not configured")
	}

	result := data.NewDingDingAPI().SendToDingDing(msg.Title, msg.Content)
	if !strings.Contains(result, "成功") {
		return errors.New("dingtalk send failed: " + result)
	}
	return nil
}
