package dialog

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth/openai"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// NewOAuthChatGPT constructs the TUI device-flow modal for ChatGPT
// sign-in. The actual flow is delegated to internal/oauth/openai —
// this adapter just translates between the OAuthProvider interface
// the modal expects and our package's device-flow functions.
func NewOAuthChatGPT(
	com *common.Common,
	isOnboarding bool,
	provider catwalk.Provider,
	model config.SelectedModel,
	modelType config.SelectedModelType,
) (*OAuth, tea.Cmd) {
	return newOAuth(com, isOnboarding, provider, model, modelType, &OAuthChatGPT{})
}

// OAuthChatGPT implements OAuthProvider over the openai device-flow
// helpers. Mirrors OAuthCopilot's shape.
type OAuthChatGPT struct {
	deviceCode *openai.DeviceCode
	cancelFunc func()
}

var _ OAuthProvider = (*OAuthChatGPT)(nil)

func (m *OAuthChatGPT) name() string {
	return "ChatGPT"
}

func (m *OAuthChatGPT) initiateAuth() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dc, err := openai.RequestDeviceCode(ctx)
	if err != nil {
		return ActionOAuthErrored{Error: fmt.Errorf("failed to initiate device auth: %w", err)}
	}

	m.deviceCode = dc

	return ActionInitiateOAuth{
		// DeviceCode here is the opaque polling state — for ChatGPT
		// that's the device_auth_id field. The user types UserCode
		// at VerificationURL; we poll until ExpiresIn elapses.
		DeviceCode:      dc.DeviceAuthID,
		UserCode:        dc.UserCode,
		VerificationURL: dc.VerificationURL,
		ExpiresIn:       int(openai.DeviceFlowTimeout / time.Second),
		Interval:        int(dc.Interval / time.Second),
	}
}

func (m *OAuthChatGPT) startPolling(_ string, _ int) tea.Cmd {
	// The interface hands us deviceCode + expiresIn but we stored the
	// full *openai.DeviceCode (including UserCode) on initiate, so
	// reuse that — PollForDeviceToken needs both device_auth_id and
	// user_code in the POST body.
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelFunc = cancel

		token, err := openai.PollForDeviceToken(ctx, m.deviceCode)
		if err != nil {
			if ctx.Err() != nil {
				return nil // cancelled, don't report error.
			}
			return ActionOAuthErrored{Error: err}
		}

		return ActionCompleteOAuth{Token: token}
	}
}

func (m *OAuthChatGPT) stopPolling() tea.Msg {
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
	return nil
}
