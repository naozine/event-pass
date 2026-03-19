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

type AttendeeHandler struct {
	Queries *database.Queries
}

func NewAttendeeHandler(queries *database.Queries) *AttendeeHandler {
	return &AttendeeHandler{Queries: queries}
}

func (h *AttendeeHandler) MyRegistrations(c echo.Context) error {
	ctx := c.Request().Context()
	userID := appcontext.GetUserID(ctx)

	registrations, err := h.Queries.ListRegistrationsByUser(ctx, userID)
	if err != nil {
		logger.Error("failed to list registrations", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load registrations")
	}

	return renderPage(c, "参加登録一覧", components.MyRegistrations(registrations))
}

func (h *AttendeeHandler) ShowPass(c echo.Context) error {
	ctx := c.Request().Context()
	userID := appcontext.GetUserID(ctx)

	regID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	reg, err := h.Queries.GetRegistrationByID(ctx, regID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Registration not found")
	}

	if reg.UserID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	event, err := h.Queries.GetEvent(ctx, reg.EventID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Event not found")
	}

	return renderPage(c, "イベントパス", components.DigitalPass(event, reg))
}

func (h *AttendeeHandler) CancelRegistration(c echo.Context) error {
	ctx := c.Request().Context()
	userID := appcontext.GetUserID(ctx)

	regID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	reg, err := h.Queries.GetRegistrationByID(ctx, regID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Registration not found")
	}

	if reg.UserID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	_, err = h.Queries.UpdateRegistrationStatus(ctx, database.UpdateRegistrationStatusParams{
		ID:     regID,
		Status: "cancelled",
	})
	if err != nil {
		logger.Error("failed to cancel registration", "error", err, "id", regID)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to cancel registration")
	}

	return c.Redirect(http.StatusSeeOther, "/my")
}
