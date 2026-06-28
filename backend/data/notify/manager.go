package notify

import (
	"context"
	"fmt"

	"go-stock/backend/logger"
)

// Manager holds all registered notification channels and dispatches messages.
type Manager struct {
	notifiers []Notifier
}

// NewManager initializes a Manager with all supported notifiers.
func NewManager() *Manager {
	return &Manager{
		notifiers: []Notifier{
			newDingTalkNotifier(),
			newWxworkNotifier(),
			newFeishuNotifier(),
			newTelegramNotifier(),
			newEmailNotifier(),
		},
	}
}

// SendAll sends the message to every enabled notifier. Errors are logged and
// do not stop delivery to other channels.
func (m *Manager) SendAll(ctx context.Context, msg Message) {
	for _, n := range m.notifiers {
		if !n.IsEnabled() {
			continue
		}
		if err := n.Send(ctx, msg); err != nil {
			logger.SugaredLogger.Errorf("notify %s failed: %v", n.ChannelType(), err)
		}
	}
}

// SendTo sends the message to the specified channel.
func (m *Manager) SendTo(ctx context.Context, channel ChannelType, msg Message) error {
	for _, n := range m.notifiers {
		if n.ChannelType() != channel {
			continue
		}
		if !n.IsEnabled() {
			return fmt.Errorf("channel %s is not enabled", channel)
		}
		return n.Send(ctx, msg)
	}
	return fmt.Errorf("unknown channel: %s", channel)
}
