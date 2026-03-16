package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/naozine/nz-magic-link/magiclink"

	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/logger"
	"github.com/naozine/project_crud_with_auth_tmpl/web/components"
	"github.com/naozine/project_crud_with_auth_tmpl/web/layouts"
)

// SetupHandler handles initial admin setup when no users exist
type SetupHandler struct {
	Queries *database.Queries
	ML      *magiclink.MagicLink
}

func NewSetupHandler(queries *database.Queries, ml *magiclink.MagicLink) *SetupHandler {
	return &SetupHandler{Queries: queries, ML: ml}
}

// hasUsers checks if any users exist in the database
func (h *SetupHandler) hasUsers(c echo.Context) (bool, error) {
	count, err := h.Queries.CountUsers(c.Request().Context())
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// SetupPage shows the initial setup page (only when no users exist)
func (h *SetupHandler) SetupPage(c echo.Context) error {
	hasUsers, err := h.hasUsers(c)
	if err != nil {
		logger.Error("Failed to count users", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "データベースエラーが発生しました")
	}

	if hasUsers {
		// Users exist, redirect to login
		return c.Redirect(http.StatusSeeOther, "/auth/login")
	}

	content := components.SetupForm("")
	return layouts.Base("初期セットアップ", content).Render(c.Request().Context(), c.Response().Writer)
}

// CreateInitialAdmin creates the first admin user and returns setup for passkey registration
func (h *SetupHandler) CreateInitialAdmin(c echo.Context) error {
	hasUsers, err := h.hasUsers(c)
	if err != nil {
		logger.Error("Failed to count users", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Database error"})
	}

	if hasUsers {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "セットアップは既に完了しています"})
	}

	// Get email from request
	var req struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if req.Email == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "メールアドレスは必須です"})
	}

	if req.Name == "" {
		req.Name = "Admin"
	}

	// Create the admin user
	_, err = h.Queries.CreateUser(c.Request().Context(), database.CreateUserParams{
		Email:    req.Email,
		Name:     req.Name,
		Role:     "admin",
		IsActive: true,
	})
	if err != nil {
		logger.Error("Failed to create admin user", "error", err, "email", req.Email)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "ユーザーの作成に失敗しました"})
	}

	logger.Info("Initial admin user created", "email", req.Email)

	// Return success - client will proceed to passkey registration
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"email":   req.Email,
		"message": "管理者ユーザーを作成しました。パスキーを登録してください。",
	})
}
