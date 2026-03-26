package reactor

import (
	"context"
	"fmt"

	"github.com/synapbus/synapbus/internal/messaging"
)

// DMFailureNotifier sends system DMs to agent owners on job failure.
type DMFailureNotifier struct {
	msgService *messaging.MessagingService
}

// NewDMFailureNotifier creates a new failure notifier.
func NewDMFailureNotifier(msgService *messaging.MessagingService) *DMFailureNotifier {
	return &DMFailureNotifier{msgService: msgService}
}

// NotifyFailure sends a system DM to the agent's owner with error details.
func (n *DMFailureNotifier) NotifyFailure(ctx context.Context, ownerAgentName, agentName, triggerFrom, triggerEvent string, durationMs int64, errorSummary string) error {
	durationStr := "< 1s"
	if durationMs > 0 {
		secs := durationMs / 1000
		if secs >= 60 {
			durationStr = fmt.Sprintf("%dm%ds", secs/60, secs%60)
		} else {
			durationStr = fmt.Sprintf("%ds", secs)
		}
	}

	body := fmt.Sprintf(
		"⚠️ **Reactive run failed** for **%s**\n\n"+
			"**Trigger**: %s from %s\n"+
			"**Duration**: %s\n"+
			"**Error**: %s\n\n"+
			"View details in Agent Runs page.",
		agentName, triggerEvent, triggerFrom, durationStr, truncateError(errorSummary, 500),
	)

	_, err := n.msgService.SendMessage(ctx, "system", ownerAgentName, body, messaging.SendOptions{
		Priority: 7,
	})
	return err
}

func truncateError(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
