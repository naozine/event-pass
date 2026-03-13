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

	// We need full user object for the form
	user, err := h.Queries.GetUserByEmail(ctx, email)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "User not found")
	}

	// Check passkey status again from DB to be sure?
	// Context has it, but let's trust context middleware which runs on every request.
	// Actually, context passkey check is basic.

	return renderPage(c, "マイページ", components.Profile(user, hasPasskey))
}

func (h *ProfileHandler) UpdateProfile(c echo.Context) error {
	ctx := c.Request().Context()
	email, _, _ := appcontext.GetUser(ctx)

	// Fetch current user to get ID and other fields we aren't changing (Role, IsActive)
	currentUser, err := h.Queries.GetUserByEmail(ctx, email)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "User not found")
	}

	newName := c.FormValue("name")
	if newName == "" {
		return c.String(http.StatusBadRequest, "Name is required")
	}

	_, err = h.Queries.UpdateUser(ctx, database.UpdateUserParams{
		Name:     newName,
		Role:     currentUser.Role,     // Keep existing role
		IsActive: currentUser.IsActive, // Keep existing status
		ID:       currentUser.ID,
	})

	if err != nil {
		logger.Error("Failed to update profile", "error", err, "email", email)
		return c.String(http.StatusInternalServerError, "Failed to update profile")
	}

	// Reload the page to show updated info
	// Or redirect to /profile
	return c.Redirect(http.StatusSeeOther, "/profile")
}

func (h *ProfileHandler) DeletePasskeys(c echo.Context) error {
	ctx := c.Request().Context()
	email, _, _ := appcontext.GetUser(ctx)

	// Get all credentials for the user
	creds, err := h.ML.DB.GetPasskeyCredentialsByUserID(email)
	if err != nil {
		logger.Error("Failed to get passkeys", "error", err, "email", email)
		return c.String(http.StatusInternalServerError, "Failed to get passkeys")
	}

	// Delete each credential
	for _, cred := range creds {
		if err := h.ML.DB.DeletePasskeyCredential(cred.ID); err != nil {
			logger.Error("Failed to delete passkey", "error", err, "credentialID", cred.ID)
			// Continue deleting others even if one fails
		}
	}

	// Use HX-Redirect to force full page reload
	c.Response().Header().Set("HX-Redirect", "/profile")
	return c.NoContent(http.StatusOK)
}
