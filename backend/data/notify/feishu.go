package notify

import (
	"context"
	"errors"
	"fmt"

	"go-stock/backend/data"
)

// feishuPayload is the request body for Feishu robot webhooks.
type feishuPayload struct {
	MsgType string     `json:"msg_type"`
	Card    feishuCard `json:"card"`
}

// feishuCard is the interactive card section of a Feishu payload.
type feishuCard struct {
	Header   feishuHeader    `json:"header"`
	Elements []feishuElement `json:"elements"`
}

// feishuHeader is the card header.
type feishuHeader struct {
	Title feishuTitle `json:"title"`
}

// feishuTitle is the header title.
type feishuTitle struct {
	Tag     string `json:"tag"`
	Content string `json:"content"`
}

// feishuElement is a card element.
type feishuElement struct {
	Tag     string `json:"tag"`
	Content string `json:"content"`
}

// feishuNotifier sends notifications through Feishu robot webhooks.
type feishuNotifier struct{}

// newFeishuNotifier creates a new Feishu notifier.
func newFeishuNotifier() *feishuNotifier {
	return &feishuNotifier{}
}

// ChannelType returns the Feishu channel identifier.
func (f *feishuNotifier) ChannelType() ChannelType {
	return ChannelFeishu
}

// IsEnabled reports whether Feishu push is enabled in settings.
func (f *feishuNotifier) IsEnabled() bool {
	return data.GetSettingConfig().FeishuPushEnable
}

// Send delivers the message through Feishu.
func (f *feishuNotifier) Send(ctx context.Context, msg Message) error {
	webhook := data.GetSettingConfig().FeishuRobot
	if webhook == "" {
		return errors.New("feishu robot webhook is not configured")
	}

	payload := feishuPayload{
		MsgType: "interactive",
		Card: feishuCard{
			Header: feishuHeader{
				Title: feishuTitle{
					Tag:     "plain_text",
					Content: msg.Title,
				},
			},
			Elements: []feishuElement{
				{
					Tag:     "markdown",
					Content: msg.Content,
				},
			},
		},
	}

	resp, err := data.SharedHTTPClient.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		Post(webhook)
	if err != nil {
		return fmt.Errorf("feishu send failed: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("feishu send failed: %s", resp.String())
	}
	return nil
}
