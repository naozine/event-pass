package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/naozine/nz-magic-link/magiclink"

	"github.com/naozine/project_crud_with_auth_tmpl/web/components"
)

type AuthHandler struct {
	ML *magiclink.MagicLink
}

func NewAuthHandler(ml *magiclink.MagicLink) *AuthHandler {
	return &AuthHandler{ML: ml}
}

func (h *AuthHandler) LoginPage(c echo.Context) error {
	_, isLoggedIn := h.ML.GetUserID(c)
	if isLoggedIn {
		return c.Redirect(http.StatusSeeOther, "/projects")
	}

	errorMessage := c.QueryParam("error_description")
	// If the error is specifically about a used token, make it more user-friendly (optional, but good UX)
	// The library returns "token has already been used" in error_description
	if c.QueryParam("error") == "token_used" {
		errorMessage = "このログインリンクは既に使用されています。もう一度メールアドレスを入力して新しいリンクを取得してください。"
	} else if errorMessage == "token has expired" {
		errorMessage = "ログインリンクの有効期限が切れています。もう一度お試しください。"
	} else if errorMessage == "invalid token" {
		errorMessage = "無効なログインリンクです。"
	}

	return renderPage(c, "ログイン", components.LoginForm(errorMessage))
}
