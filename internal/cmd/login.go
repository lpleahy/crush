package cmd

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/clipboard"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/charmbracelet/crush/internal/oauth/hyper"
	"github.com/charmbracelet/crush/internal/oauth/openai"
	"github.com/charmbracelet/x/ansi"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Aliases: []string{"auth"},
	Use:     "login [platform]",
	Short:   "Login Crush to a platform",
	Long: `Login Crush to a specified platform.
The platform should be provided as an argument.
Available platforms are: hyper, copilot, chatgpt.`,
	Example: `
# Authenticate with Charm Hyper
crush login

# Authenticate with GitHub Copilot
crush login copilot

# Authenticate with a ChatGPT subscription
crush login chatgpt

# Force re-authentication even if already logged in
crush login -f copilot
  `,
	ValidArgs: []cobra.Completion{
		"hyper",
		"copilot",
		"github",
		"github-copilot",
		"chatgpt",
		"openai-chatgpt",
	},
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, ws, cleanup, err := connectToServer(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		progressEnabled := ws.Config.Options.Progress == nil || *ws.Config.Options.Progress
		if progressEnabled && supportsProgressBar() {
			_, _ = fmt.Fprintf(os.Stderr, ansi.SetIndeterminateProgressBar)
			defer func() { _, _ = fmt.Fprintf(os.Stderr, ansi.ResetProgressBar) }()
		}

		provider := "hyper"
		if len(args) > 0 {
			provider = args[0]
		}
		force, _ := cmd.Flags().GetBool("force")
		noBrowser, _ := cmd.Flags().GetBool("no-browser")
		switch provider {
		case "hyper":
			return loginHyper(c, ws.ID, force)
		case "copilot", "github", "github-copilot":
			return loginCopilot(c, ws.ID, force)
		case "chatgpt", "openai-chatgpt":
			return loginChatGPT(c, ws.ID, force, noBrowser)
		default:
			return fmt.Errorf("unknown platform: %s", args[0])
		}
	},
}

func init() {
	loginCmd.Flags().BoolP("force", "f", false, "Force re-authentication even if already logged in")
	loginCmd.Flags().Bool("no-browser", false, "Don't try to open a browser automatically (for SSH/headless use)")
}

func loginHyper(c *client.Client, wsID string, force bool) error {
	ctx := getLoginContext()

	if !force {
		cfg, err := c.GetConfig(ctx, wsID)
		if err == nil && cfg != nil {
			if pc, ok := cfg.Providers.Get("hyper"); ok && pc.OAuthToken != nil {
				fmt.Println("You are already logged in to Hyper.")
				fmt.Println("Use --force to re-authenticate.")
				return nil
			}
		}
	}

	resp, err := hyper.InitiateDeviceAuth(ctx)
	if err != nil {
		return err
	}

	clipboard.WriteText(resp.UserCode)
	fmt.Println("The following code should be on clipboard already:")

	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Bold(true).Render(resp.UserCode))
	fmt.Println()
	fmt.Println("Press enter to open this URL, and then paste it there:")
	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Hyperlink(resp.VerificationURL, "id=hyper").Render(resp.VerificationURL))
	fmt.Println()
	waitEnter()
	if err := browser.OpenURL(resp.VerificationURL); err != nil {
		fmt.Println("Could not open the URL. You'll need to manually open the URL in your browser.")
	}

	fmt.Println("Exchanging authorization code...")
	refreshToken, err := hyper.PollForToken(ctx, resp.DeviceCode, resp.ExpiresIn)
	if err != nil {
		return err
	}

	fmt.Println("Exchanging refresh token for access token...")
	token, err := hyper.ExchangeToken(ctx, refreshToken)
	if err != nil {
		return err
	}

	fmt.Println("Verifying access token...")
	introspect, err := hyper.IntrospectToken(ctx, token.AccessToken)
	if err != nil {
		return fmt.Errorf("token introspection failed: %w", err)
	}
	if !introspect.Active {
		return fmt.Errorf("access token is not active")
	}

	if err := cmp.Or(
		c.SetConfigField(ctx, wsID, config.ScopeGlobal, "providers.hyper.api_key", token.AccessToken),
		c.SetConfigField(ctx, wsID, config.ScopeGlobal, "providers.hyper.oauth", token),
	); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with Hyper!")
	return nil
}

func loginCopilot(c *client.Client, wsID string, force bool) error {
	loginCtx := getLoginContext()

	if !force {
		cfg, err := c.GetConfig(loginCtx, wsID)
		if err == nil && cfg != nil {
			if pc, ok := cfg.Providers.Get("copilot"); ok && pc.OAuthToken != nil {
				fmt.Println("You are already logged in to GitHub Copilot.")
				fmt.Println("Use --force to re-authenticate.")
				return nil
			}
		}
	}

	diskToken, hasDiskToken := copilot.RefreshTokenFromDisk()
	var token *oauth.Token

	switch {
	case hasDiskToken:
		fmt.Println("Found existing GitHub Copilot token on disk. Using it to authenticate...")

		t, err := copilot.RefreshToken(loginCtx, diskToken)
		if err != nil {
			return fmt.Errorf("unable to refresh token from disk: %w", err)
		}
		token = t
	default:
		fmt.Println("Requesting device code from GitHub...")
		dc, err := copilot.RequestDeviceCode(loginCtx)
		if err != nil {
			return err
		}

		fmt.Println()
		fmt.Println("Open the following URL and follow the instructions to authenticate with GitHub Copilot:")
		fmt.Println()
		fmt.Println(lipgloss.NewStyle().Hyperlink(dc.VerificationURI, "id=copilot").Render(dc.VerificationURI))
		fmt.Println()
		fmt.Println("Code:", lipgloss.NewStyle().Bold(true).Render(dc.UserCode))
		fmt.Println()
		fmt.Println("Waiting for authorization...")

		t, err := copilot.PollForToken(loginCtx, dc)
		if err == copilot.ErrNotAvailable {
			fmt.Println()
			fmt.Println("GitHub Copilot is unavailable for this account. To signup, go to the following page:")
			fmt.Println()
			fmt.Println(lipgloss.NewStyle().Hyperlink(copilot.SignupURL, "id=copilot-signup").Render(copilot.SignupURL))
			fmt.Println()
			fmt.Println("You may be able to request free access if eligible. For more information, see:")
			fmt.Println()
			fmt.Println(lipgloss.NewStyle().Hyperlink(copilot.FreeURL, "id=copilot-free").Render(copilot.FreeURL))
		}
		if err != nil {
			return err
		}
		token = t
	}

	if err := cmp.Or(
		c.SetConfigField(loginCtx, wsID, config.ScopeGlobal, "providers.copilot.api_key", token.AccessToken),
		c.SetConfigField(loginCtx, wsID, config.ScopeGlobal, "providers.copilot.oauth", token),
	); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with GitHub Copilot!")
	return nil
}

func loginChatGPT(c *client.Client, wsID string, force, noBrowser bool) error {
	loginCtx := getLoginContext()

	if !force {
		if cfg, err := c.GetConfig(loginCtx, wsID); err == nil && cfg != nil {
			if pc, ok := cfg.Providers.Get("chatgpt"); ok && pc.OAuthToken != nil {
				fmt.Println("You are already logged in to ChatGPT.")
				fmt.Println("Use --force to re-authenticate.")
				return nil
			}
		}
	}

	headless := noBrowser || isSSHSession()

	var browserOpener openai.BrowserOpener
	if !headless {
		browserOpener = browserOpenerFunc(browser.OpenURL)
	}

	fmt.Println("Starting ChatGPT sign-in...")

	result, err := openai.Authorize(loginCtx, openai.AuthorizeOptions{
		Browser: browserOpener,
		OnReady: func(authURL string) {
			fmt.Println()
			fmt.Println("Open the following URL in your browser to sign in:")
			fmt.Println(lipgloss.NewStyle().Hyperlink(authURL, "id=chatgpt").Render(authURL))
			fmt.Println()
			if headless {
				fmt.Println("After signing in, your browser will try to redirect to")
				fmt.Println("http://localhost:1455. If that URL won't load on this machine,")
				fmt.Println("paste the full localhost URL it tried to load here:")
				go pasteCallback()
			} else {
				fmt.Println("Waiting for authorization...")
			}
		},
	})
	if err != nil {
		return err
	}

	fmt.Println("Exchanging authorization code...")
	token, err := openai.TokenExchange(loginCtx, result.Code, result.Verifier)
	if err != nil {
		return err
	}

	if err := cmp.Or(
		c.SetConfigField(loginCtx, wsID, config.ScopeGlobal, "providers.chatgpt.api_key", token.AccessToken),
		c.SetConfigField(loginCtx, wsID, config.ScopeGlobal, "providers.chatgpt.oauth", token),
	); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("You're now authenticated with ChatGPT!")
	return nil
}

type browserOpenerFunc func(url string) error

func (f browserOpenerFunc) Open(url string) error { return f(url) }

func isSSHSession() bool {
	return os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != ""
}

func pasteCallback() {
	var pasted string
	if _, err := fmt.Scanln(&pasted); err != nil {
		return
	}
	u, err := url.Parse(pasted)
	if err != nil || u.RawQuery == "" {
		return
	}
	// Replay the callback against our own loopback listener. The
	// handler running inside openai.Authorize picks up the code.
	// Use the same redirect URI Authorize advertised so any future
	// change there flows through automatically.
	_, _ = http.Get(openai.DefaultRedirectURI + "?" + u.RawQuery)
}

func getLoginContext() context.Context {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	go func() {
		<-ctx.Done()
		cancel()
		os.Exit(1)
	}()
	return ctx
}

func waitEnter() {
	_, _ = fmt.Scanln()
}
