package notify

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go-stock/backend/data"
)

// telegramPayload is the request body for Telegram Bot API sendMessage.
type telegramPayload struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// telegramNotifier sends notifications through the Telegram Bot API.
type telegramNotifier struct{}

// newTelegramNotifier creates a new Telegram notifier.
func newTelegramNotifier() *telegramNotifier {
	return &telegramNotifier{}
}

// ChannelType returns the Telegram channel identifier.
func (t *telegramNotifier) ChannelType() ChannelType {
	return ChannelTelegram
}

// IsEnabled reports whether Telegram push is enabled in settings.
func (t *telegramNotifier) IsEnabled() bool {
	return data.GetSettingConfig().TelegramPushEnable
}

// Send delivers the message through Telegram.
func (t *telegramNotifier) Send(ctx context.Context, msg Message) error {
	token := data.GetSettingConfig().TelegramBotToken
	chatID := data.GetSettingConfig().TelegramChatID
	if token == "" || chatID == "" {
		return errors.New("telegram bot token or chat id is not configured")
	}

	// Telegram's Markdown parser only supports a limited subset of Markdown.
	// Strip characters that are likely to cause parsing errors while keeping
	// the message readable.
	text := msg.Title + "\n\n" + simplifyMarkdown(msg.Content)

	payload := telegramPayload{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "Markdown",
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	resp, err := data.SharedHTTPClient.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		Post(apiURL)
	if err != nil {
		return fmt.Errorf("telegram send failed: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("telegram send failed: %s", resp.String())
	}
	return nil
}

func simplifyMarkdown(content string) string {
	for i := 6; i >= 1; i-- {
		prefix := strings.Repeat("#", i) + " "
		content = strings.ReplaceAll(content, prefix, "*")
	}
	return content
}
