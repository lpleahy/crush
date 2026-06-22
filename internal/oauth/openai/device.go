package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
)

// DeviceFlowTimeout is the hard wall-clock deadline for the user to
// complete the device flow. Mirrors Codex CLI's 15-minute cap.
const DeviceFlowTimeout = 15 * time.Minute

// minDevicePollInterval clamps server-provided poll intervals from
// below — a runaway "interval: 0" would otherwise hammer the
// endpoint.
const minDevicePollInterval = 1 * time.Second

// DeviceCode is what the caller shows the user: a verification URL
// to visit and a one-time code to type in. DeviceAuthID is opaque
// state for polling.
type DeviceCode struct {
	DeviceAuthID    string
	UserCode        string
	VerificationURL string
	Interval        time.Duration
}

// userCodeResp matches the /deviceauth/usercode response. The Codex
// server has been observed returning the code as either user_code or
// usercode and the interval as either a JSON number or a quoted
// string; both shapes are decoded here.
type userCodeResp struct {
	DeviceAuthID string          `json:"device_auth_id"`
	UserCode     string          `json:"user_code"`
	UserCodeAlt  string          `json:"usercode"`
	Interval     intervalSeconds `json:"interval"`
}

type intervalSeconds int

func (s *intervalSeconds) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		*s = 0
		return nil
	}
	if data[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		if str == "" {
			*s = 0
			return nil
		}
		n, err := strconv.Atoi(str)
		if err != nil {
			return err
		}
		*s = intervalSeconds(n)
		return nil
	}
	var n int
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*s = intervalSeconds(n)
	return nil
}

// codeSuccessResp is what /deviceauth/token returns once the user
// finishes consent. The server pre-generates the PKCE pair; the
// client just forwards them to the standard token exchange.
type codeSuccessResp struct {
	AuthorizationCode string `json:"authorization_code"`
	CodeChallenge     string `json:"code_challenge"`
	CodeVerifier      string `json:"code_verifier"`
}

// RequestDeviceCode initiates a device-flow login. The returned
// DeviceCode carries everything the caller needs to prompt the user.
func RequestDeviceCode(ctx context.Context) (*DeviceCode, error) {
	return DefaultClient.RequestDeviceCode(ctx)
}

// PollForDeviceToken blocks until the user completes consent or the
// 15-minute deadline expires, then exchanges the resulting code for
// access + refresh tokens. ctx cancellation aborts the wait.
func PollForDeviceToken(ctx context.Context, dc *DeviceCode) (*oauth.Token, error) {
	return DefaultClient.PollForDeviceToken(ctx, dc)
}

func (c *Client) RequestDeviceCode(ctx context.Context) (*DeviceCode, error) {
	body, _ := json.Marshal(map[string]string{"client_id": c.ClientID})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.DeviceUserCodeURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("device code login is not enabled for this account (workspace admins may have disabled it; try the browser flow without --no-browser)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("usercode endpoint %d: %s", resp.StatusCode, string(respBody))
	}

	var ur userCodeResp
	if err := json.Unmarshal(respBody, &ur); err != nil {
		return nil, fmt.Errorf("decode usercode response: %w", err)
	}

	code := ur.UserCode
	if code == "" {
		code = ur.UserCodeAlt
	}
	if ur.DeviceAuthID == "" || code == "" {
		return nil, fmt.Errorf("usercode response missing device_auth_id or user_code")
	}

	interval := time.Duration(ur.Interval) * time.Second
	if interval < minDevicePollInterval {
		interval = minDevicePollInterval
	}

	return &DeviceCode{
		DeviceAuthID:    ur.DeviceAuthID,
		UserCode:        code,
		VerificationURL: c.DeviceVerificationURL,
		Interval:        interval,
	}, nil
}

func (c *Client) PollForDeviceToken(ctx context.Context, dc *DeviceCode) (*oauth.Token, error) {
	if dc == nil {
		return nil, fmt.Errorf("PollForDeviceToken: nil device code")
	}

	reqBody, _ := json.Marshal(map[string]string{
		"device_auth_id": dc.DeviceAuthID,
		"user_code":      dc.UserCode,
	})

	interval := dc.Interval
	if interval < minDevicePollInterval {
		interval = minDevicePollInterval
	}

	deadlineCtx, cancel := context.WithTimeout(ctx, DeviceFlowTimeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-deadlineCtx.Done():
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("device flow timed out after %s", DeviceFlowTimeout)
		case <-ticker.C:
		}

		success, err := c.pollDeviceOnce(deadlineCtx, reqBody)
		if err != nil {
			return nil, err
		}
		if success != nil {
			return c.exchangeDeviceCode(ctx, success.AuthorizationCode, success.CodeVerifier)
		}
	}
}

// pollDeviceOnce returns (success, nil) when the user finished
// consent, (nil, nil) when the server says keep waiting (403/404),
// and (nil, err) on hard errors.
func (c *Client) pollDeviceOnce(ctx context.Context, body []byte) (*codeSuccessResp, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.DeviceTokenURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return nil, err
		}
		var cs codeSuccessResp
		if err := json.Unmarshal(respBody, &cs); err != nil {
			return nil, fmt.Errorf("decode device token response: %w", err)
		}
		if cs.AuthorizationCode == "" || cs.CodeVerifier == "" {
			return nil, fmt.Errorf("device token response missing authorization_code or code_verifier")
		}
		return &cs, nil
	case http.StatusForbidden, http.StatusNotFound:
		return nil, nil
	default:
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, fmt.Errorf("device token endpoint %d: %s", resp.StatusCode, string(respBody))
	}
}

// exchangeDeviceCode hits the standard /oauth/token endpoint with
// the auth code and server-supplied PKCE verifier, returning a
// fully-populated oauth.Token. We must pass the device-flow
// redirect URI (auth.openai.com/deviceauth/callback) not the
// loopback URI used for PKCE.
func (c *Client) exchangeDeviceCode(ctx context.Context, code, verifier string) (*oauth.Token, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {c.ClientID},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {c.DeviceRedirectURI},
	}
	return c.postToken(ctx, form)
}
