package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/naozine/nz-magic-link/magiclink"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/appcontext"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/logger"
	"github.com/naozine/project_crud_with_auth_tmpl/web/components"
)

type ProfileHandler struct {
	Queries *database.Queries
	ML      *magiclink.MagicLink
}

func NewProfileHandler(queries *database.Queries, ml *magiclink.MagicLink) *ProfileHandler {
	return &ProfileHandler{Queries: queries, ML: ml}
}

func (h *ProfileHandler) ShowProfile(c echo.Context) error {
	ctx := c.Request().Context()

	email, _, hasPasskey := appcontext.GetUser(ctx)

	user, err := h.Queries.GetUserByEmail(ctx, email)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "ユーザーが見つかりません")
	}

	return renderPage(c, "マイページ", components.Profile(user, hasPasskey))
}

func (h *ProfileHandler) UpdateProfile(c echo.Context) error {
	ctx := c.Request().Context()
	email, _, _ := appcontext.GetUser(ctx)

	currentUser, err := h.Queries.GetUserByEmail(ctx, email)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "ユーザーが見つかりません")
	}

	newName := c.FormValue("name")
	if newName == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "名前は必須です")
	}

	_, err = h.Queries.UpdateUser(ctx, database.UpdateUserParams{
		Name:     newName,
		Role:     currentUser.Role,
		IsActive: currentUser.IsActive,
		ID:       currentUser.ID,
	})
	if err != nil {
		logger.Error("プロフィール更新に失敗", "error", err, "email", email)
		return echo.NewHTTPError(http.StatusInternalServerError, "プロフィールの更新に失敗しました")
	}

	return c.Redirect(http.StatusSeeOther, "/profile")
}

func (h *ProfileHandler) DeletePasskeys(c echo.Context) error {
	ctx := c.Request().Context()
	email, _, _ := appcontext.GetUser(ctx)

	creds, err := h.ML.DB.GetPasskeyCredentialsByUserID(email)
	if err != nil {
		logger.Error("パスキーの取得に失敗", "error", err, "email", email)
		return echo.NewHTTPError(http.StatusInternalServerError, "パスキーの取得に失敗しました")
	}

	for _, cred := range creds {
		if err := h.ML.DB.DeletePasskeyCredential(cred.ID); err != nil {
			logger.Error("パスキーの削除に失敗", "error", err, "credentialID", cred.ID)
		}
	}

	return c.Redirect(http.StatusSeeOther, "/profile")
}
