package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/appcontext"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/logger"
	"github.com/naozine/project_crud_with_auth_tmpl/web/components"
)

type AdminHandler struct {
	Queries *database.Queries
}

func NewAdminHandler(queries *database.Queries) *AdminHandler {
	return &AdminHandler{Queries: queries}
}

func (h *AdminHandler) ListUsers(c echo.Context) error {
	users, err := h.Queries.ListUsers(c.Request().Context())
	if err != nil {
		logger.Error("ユーザー一覧の取得に失敗", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "ユーザー一覧の取得に失敗しました")
	}
	return renderPage(c, "ユーザー管理", components.AdminUserList(users))
}

func (h *AdminHandler) NewUserPage(c echo.Context) error {
	return renderPage(c, "ユーザー登録", components.AdminUserForm())
}

func (h *AdminHandler) CreateUser(c echo.Context) error {
	name := c.FormValue("name")
	email := c.FormValue("email")
	role := c.FormValue("role")

	if name == "" || email == "" || role == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "名前・メールアドレス・ロールは必須です")
	}

	if role != "admin" && role != "editor" && role != "viewer" {
		return echo.NewHTTPError(http.StatusBadRequest, "無効なロールです")
	}

	_, err := h.Queries.CreateUser(c.Request().Context(), database.CreateUserParams{
		Email:    email,
		Name:     name,
		Role:     role,
		IsActive: true,
	})
	if err != nil {
		logger.Error("ユーザー作成に失敗", "error", err, "email", email)
		return echo.NewHTTPError(http.StatusInternalServerError, "ユーザーの作成に失敗しました")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/users")
}

func (h *AdminHandler) EditUserPage(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "無効なIDです")
	}

	user, err := h.Queries.GetUserByID(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "ユーザーが見つかりません")
	}

	return renderPage(c, "ユーザー編集", components.AdminUserEdit(user))
}

func (h *AdminHandler) UpdateUser(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "無効なIDです")
	}

	name := c.FormValue("name")
	role := c.FormValue("role")
	isActive := c.FormValue("status") == "active"

	_, err = h.Queries.UpdateUser(c.Request().Context(), database.UpdateUserParams{
		Name:     name,
		Role:     role,
		IsActive: isActive,
		ID:       id,
	})
	if err != nil {
		logger.Error("ユーザー更新に失敗", "error", err, "id", id)
		return echo.NewHTTPError(http.StatusInternalServerError, "ユーザーの更新に失敗しました")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/users")
}

func (h *AdminHandler) DeleteUser(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "無効なIDです")
	}

	// 自分自身の削除を防止
	currentUserID := appcontext.GetUserID(c.Request().Context())
	if id == currentUserID {
		return echo.NewHTTPError(http.StatusBadRequest, "自分自身を削除することはできません")
	}

	err = h.Queries.DeleteUser(c.Request().Context(), id)
	if err != nil {
		logger.Error("ユーザー削除に失敗", "error", err, "id", id)
		return echo.NewHTTPError(http.StatusInternalServerError, "ユーザーの削除に失敗しました")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/users")
}
