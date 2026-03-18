package main

import (
	"os"

	"github.com/naozine/nz-magic-link/magiclink"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/appconfig"
)

// ConfigureBusinessSettings allows customization of MagicLink config and App Name
func ConfigureBusinessSettings(config *magiclink.Config) {
	config.RedirectURL = "/events"         // Redirect to event list after login
	config.WebAuthnRedirectURL = "/events" // Redirect to event list after passkey login

	// 負荷テスト用: DISABLE_RATE_LIMITING=true でレート制限を無効化
	if os.Getenv("DISABLE_RATE_LIMITING") == "true" {
		config.DisableRateLimiting = true
	}

	// Set Application Name
	appconfig.AppName = "Event Pass"
}
