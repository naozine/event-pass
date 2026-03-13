package main

import (
	"github.com/naozine/nz-magic-link/magiclink"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/appconfig"
)

// ConfigureBusinessSettings allows customization of MagicLink config and App Name
func ConfigureBusinessSettings(config *magiclink.Config) {
	config.RedirectURL = "/projects"         // Redirect to projects list after login
	config.WebAuthnRedirectURL = "/projects" // Redirect to projects list after passkey login

	// Set Application Name
	appconfig.AppName = "プロジェクト管理"
}
