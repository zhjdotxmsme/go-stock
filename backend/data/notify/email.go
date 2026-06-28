package notify

import (
	"context"
	"errors"
	"fmt"
	"net/smtp"
	"strings"

	"go-stock/backend/data"
)

// emailNotifier sends notifications through SMTP.
type emailNotifier struct{}

// newEmailNotifier creates a new email notifier.
func newEmailNotifier() *emailNotifier {
	return &emailNotifier{}
}

// ChannelType returns the email channel identifier.
func (e *emailNotifier) ChannelType() ChannelType {
	return ChannelEmail
}

// IsEnabled reports whether email push is enabled in settings.
func (e *emailNotifier) IsEnabled() bool {
	return data.GetSettingConfig().EmailPushEnable
}

// Send delivers the message through SMTP email.
func (e *emailNotifier) Send(ctx context.Context, msg Message) error {
	cfg := data.GetSettingConfig()
	host := cfg.EmailSmtpHost
	port := cfg.EmailSmtpPort
	user := cfg.EmailSmtpUser
	pass := cfg.EmailSmtpPass
	to := cfg.EmailTo

	if host == "" || port <= 0 || user == "" || pass == "" || to == "" {
		return errors.New("email smtp config is incomplete")
	}

	from := user

	addr := fmt.Sprintf("%s:%d", host, port)
	recipients := strings.Split(to, ",")
	for i := range recipients {
		recipients[i] = strings.TrimSpace(recipients[i])
	}

	headers := make([]string, 0, 4)
	headers = append(headers, "From: "+from)
	headers = append(headers, "To: "+to)
	headers = append(headers, "Subject: "+msg.Title)
	headers = append(headers, "Content-Type: text/plain; charset=\"utf-8\"")
	body := strings.Join(headers, "\r\n") + "\r\n\r\n" + msg.Content

	auth := smtp.PlainAuth("", user, pass, host)
	return smtp.SendMail(addr, auth, from, recipients, []byte(body))
}
